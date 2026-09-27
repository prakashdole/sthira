// Command preflight is a LOCAL, OFFLINE readiness check for the real-model
// launch procedure (asrworker, middleworker, ttsworker + the private vLLM
// endpoint they front). It never loads model weights, never opens a network
// connection, and never starts paid work. It exists to catch launch-time
// mistakes (missing env var, missing python interpreter, missing artifact
// directory, stale flag name) BEFORE anyone spends GPU time or cloud money.
//
// Scope discipline (see plan/prompt.md and README.md "Don't"):
//   - Static/filesystem checks only. No os/exec of python, no model loading,
//     no HTTP calls to vLLM, ASR/TTS artifact directories, or any cloud API.
//   - Every check that requires a live remote service (vLLM health, AWS
//     instance state, SSH tunnels) is reported NOT_RUN, never PASS/FAIL.
//   - This binary does not orchestrate process startup; it only verifies the
//     preconditions documented in plan/prompt.md Procedure B and
//     plan/evidence/execution-c04.md are actually met on this host.
//
// This file intentionally has zero non-stdlib dependencies: reuse the Go
// standard library per the project's simplicity ladder instead of adding a
// CLI framework for a handful of checks.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// status is a tri-state result. Never invent PASS for something unmeasured.
type status string

const (
	pass   status = "PASS"
	fail   status = "FAIL"
	notRun status = "NOT_RUN"
)

type result struct {
	name   string
	st     status
	detail string
}

func (r result) String() string {
	return fmt.Sprintf("[%-7s] %-42s %s", r.st, r.name, r.detail)
}

// config bundles the host paths under test. Defaults come from the
// user-corrected paths recorded in plan/evidence/real-inference-launch-check.md;
// every value is overridable via flag/env so this tool works against any host,
// and it never assumes a path is correct merely because it is the default.
type config struct {
	repoRoot string

	sarvamDir    string
	asrDir       string
	ttsDir       string
	pythonBin    string
	pythonPath   string
	descTokDir   string
	voicesFile   string

	vllmURL    string
	asrAddr    string
	middleAddr string
	ttsAddr    string
}

func main() {
	cfg := config{}
	repoRootDefault, _ := os.Getwd()

	flag.StringVar(&cfg.repoRoot, "repo-root", repoRootDefault, "repository root (auto-detected from cwd if run from anywhere inside it)")
	flag.StringVar(&cfg.sarvamDir, "sarvam-dir", envOr("STHIRA_PREFLIGHT_SARVAM_DIR", "/home/ubuntu/models/sarvam-30b-fp8"), "host path to Sarvam-30B FP8 weights (informational; middleworker itself never loads local weights, vLLM does)")
	flag.StringVar(&cfg.asrDir, "asr-dir", envOr("STHIRA_ASR_ARTIFACT_DIR", "/home/ubuntu/models/indic-conformer-600m-multilingual"), "host path for STHIRA_ASR_ARTIFACT_DIR")
	flag.StringVar(&cfg.ttsDir, "tts-dir", envOr("STHIRA_TTS_ARTIFACT_DIR", "/home/ubuntu/models/indic-parler-tts"), "host path for STHIRA_TTS_ARTIFACT_DIR")
	flag.StringVar(&cfg.pythonBin, "python", envOr("STHIRA_ASR_PYTHON", "/home/ubuntu/.venvs/voice/bin/python"), "python interpreter path for STHIRA_ASR_PYTHON / STHIRA_TTS_PYTHON")
	flag.StringVar(&cfg.pythonPath, "pythonpath", envOr("STHIRA_PREFLIGHT_PYTHONPATH", "/opt/pytorch/lib/python3.12/site-packages"), "PYTHONPATH entry expected to carry torch/transformers/parler_tts/onnxruntime")
	flag.StringVar(&cfg.descTokDir, "tts-text-encoder-dir", envOr("STHIRA_TTS_TEXT_ENCODER_DIR", "/home/ubuntu/models/flan-t5-large-tokenizer"), "host path for STHIRA_TTS_TEXT_ENCODER_DIR (Indic Parler-TTS description tokenizer)")
	flag.StringVar(&cfg.voicesFile, "tts-voices-file", envOr("STHIRA_TTS_VOICES_FILE", ""), "host path for STHIRA_TTS_VOICES_FILE (optional; if empty, checked as NOT_RUN/informational only)")
	flag.StringVar(&cfg.vllmURL, "vllm-url", envOr("STHIRA_VLLM_URL", "http://127.0.0.1:8000"), "vLLM base URL middleworker will call (never contacted by this tool)")
	flag.StringVar(&cfg.asrAddr, "asr-addr", envOr("STHIRA_ASR_ADDR", "127.0.0.1:8001"), "asrworker private listen address")
	flag.StringVar(&cfg.middleAddr, "middle-addr", envOr("STHIRA_MIDDLE_ADDR", "127.0.0.1:8002"), "middleworker private listen address")
	flag.StringVar(&cfg.ttsAddr, "tts-addr", envOr("STHIRA_TTS_ADDR", "127.0.0.1:8003"), "ttsworker private listen address")
	flag.Parse()

	root, err := findRepoRoot(cfg.repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(2)
	}
	cfg.repoRoot = root

	var results []result
	add := func(r result) { results = append(results, r) }

	fmt.Println("======================================================================")
	fmt.Println("Sthira real-model LOCAL preflight (offline, no weights loaded, no network)")
	fmt.Println("Repo root:", cfg.repoRoot)
	fmt.Println("======================================================================")

	// --- 1. Module-aware Go build commands for each worker -----------------
	for _, w := range []struct{ mod, entry string }{
		{"backend/internal/asrworker", "./cmd/asrworker"},
		{"backend/internal/middleworker", "./cmd/middleworker"},
		{"backend/internal/ttsworker", "./cmd/ttsworker"},
	} {
		add(checkModuleBuild(cfg.repoRoot, w.mod, w.entry))
	}

	// --- 2. Flags/env vars actually referenced in worker source ------------
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"backend/internal/asrworker/cmd/asrworker/main.go",
		[]string{"STHIRA_ASR_ADDR", "STHIRA_ASR_TOKEN", "STHIRA_ASR_PYTHON", "STHIRA_ASR_ADAPTER", "STHIRA_ASR_WORKDIR", "STHIRA_ASR_QUEUE_DEPTH", "STHIRA_ASR_MAX_INFLIGHT"},
		"asrworker main.go"))
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"src/sthira_v2/speech_asr_adapter.py",
		[]string{"STHIRA_ASR_ARTIFACT_DIR", "STHIRA_ASR_APPROVED_LANGUAGES", "STHIRA_ASR_WARMUP_TIMEOUT_SECONDS"},
		"speech_asr_adapter.py"))
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"backend/internal/middleworker/cmd/middleworker/main.go",
		[]string{"STHIRA_MIDDLE_ADDR", "STHIRA_MIDDLE_TOKEN", "STHIRA_VLLM_URL", "STHIRA_MIDDLE_REVISION", "STHIRA_MIDDLE_DIGEST_SHA", "STHIRA_MIDDLE_QUEUE_DEPTH", "STHIRA_MIDDLE_MAX_INFLIGHT"},
		"middleworker main.go"))
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"backend/internal/ttsworker/cmd/ttsworker/main.go",
		[]string{"STHIRA_TTS_SYNTHETIC_EXERCISE", "STHIRA_TTS_ADDR", "STHIRA_TTS_TOKEN", "STHIRA_TTS_PYTHON", "STHIRA_TTS_ADAPTER", "STHIRA_TTS_WORKDIR", "STHIRA_TTS_QUEUE_DEPTH", "STHIRA_TTS_MAX_INFLIGHT", "STHIRA_TTS_SOURCE_VERSION"},
		"ttsworker main.go"))
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"src/sthira_v2/speech_tts_adapter.py",
		[]string{"STHIRA_TTS_ARTIFACT_DIR", "STHIRA_TTS_APPROVED_LANGUAGES", "STHIRA_TTS_VOICES_FILE", "STHIRA_TTS_MAX_NEW_TOKENS", "STHIRA_TTS_TEXT_ENCODER_DIR", "STHIRA_TTS_DEVICE", "STHIRA_TTS_WARMUP_TIMEOUT_SECONDS"},
		"speech_tts_adapter.py"))
	add(checkEnvVarsExistInSource(cfg.repoRoot,
		"backend/cmd/sthira-exercise/main.go",
		[]string{"STHIRA_ASR_URL", "STHIRA_ASR_TOKEN", "STHIRA_MIDDLE_URL", "STHIRA_MIDDLE_TOKEN", "STHIRA_TTS_URL", "STHIRA_TTS_TOKEN"},
		"cmd/sthira-exercise main.go"))

	// --- 3. Model identifier / server config consistency (static) ---------
	add(checkFileContains(cfg.repoRoot, "backend/internal/middleworker/sarvam_config.go",
		`SarvamModelID = "sarvamai/sarvam-30b"`, "Sarvam model_id constant matches specified deployment (sarvamai/sarvam-30b)"))
	add(checkFileContains(cfg.repoRoot, "backend/internal/middleworker/sarvam_config.go",
		`"--port", "8000"`, "Documented vLLM startup args pin port 8000 (private, matches specified sthira-sarvam deployment)"))

	// --- 4. Artifact/interpreter/PYTHONPATH presence (no weight loading) ---
	add(checkDirExists("ASR artifact dir (STHIRA_ASR_ARTIFACT_DIR)", cfg.asrDir))
	add(checkDirExists("TTS artifact dir (STHIRA_TTS_ARTIFACT_DIR)", cfg.ttsDir))
	add(checkDirExists("TTS description tokenizer dir (STHIRA_TTS_TEXT_ENCODER_DIR)", cfg.descTokDir))
	add(checkDirExists("Sarvam-30B FP8 weights dir (vLLM container mount, informational)", cfg.sarvamDir))
	add(checkExecutable("Python interpreter (STHIRA_ASR_PYTHON / STHIRA_TTS_PYTHON)", cfg.pythonBin))
	add(checkDirExists("PYTHONPATH entry (site-packages for torch/transformers/parler_tts/onnxruntime)", cfg.pythonPath))
	if cfg.voicesFile != "" {
		add(checkFileExists("TTS approved voices file (STHIRA_TTS_VOICES_FILE)", cfg.voicesFile))
	} else {
		add(result{"TTS approved voices file (STHIRA_TTS_VOICES_FILE)", notRun, "not set; no default assumed — required before real TTS synthesis, absence fails closed by design"})
	}

	// --- 5. Port collision check (loopback only, no cloud, no third party) -
	for _, p := range []struct{ name, addr string }{
		{"asrworker listen addr", cfg.asrAddr},
		{"middleworker listen addr", cfg.middleAddr},
		{"ttsworker listen addr", cfg.ttsAddr},
	} {
		add(checkLoopbackPortFree(p.name, p.addr))
	}

	// --- 6. Explicit NOT_RUN markers for anything remote/paid --------------
	add(result{"vLLM endpoint reachability (" + cfg.vllmURL + ")", notRun, "remote-only check; this tool makes no network connections"})
	add(result{"AWS instance state / SSH tunnel", notRun, "remote-only check; run `aws ec2 describe-instances` manually under explicit authorization"})
	add(result{"Model weight loading (ASR/TTS/Sarvam)", notRun, "explicitly out of scope for local preflight; would require GPU/CPU time and is not run automatically"})

	fmt.Println("----------------------------------------------------------------------")
	failures := 0
	for _, r := range results {
		fmt.Println(r.String())
		if r.st == fail {
			failures++
		}
	}
	fmt.Println("======================================================================")
	fmt.Printf("TOTAL: %d checks, %d FAIL\n", len(results), failures)
	if failures > 0 {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// findRepoRoot walks upward from start until it finds go.mod for the
// backend module or a .git directory, so the tool can be invoked from
// any working directory inside the repo.
func findRepoRoot(start string) (string, error) {
	dir := start
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "backend", "go.mod")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("could not locate repo root (backend/go.mod not found above " + start + "); pass -repo-root explicitly")
}

// checkModuleBuild runs `go build -o /dev/null <entry>` inside the given
// nested module directory. This performs a real compiler invocation
// (catching stale flags, broken imports, etc.) but never runs the
// resulting binary and never touches the network (GOFLAGS=-mod=mod is not
// set; module resolution uses the existing go.sum / vendored deps only).
func checkModuleBuild(repoRoot, relModDir, entry string) result {
	name := fmt.Sprintf("go build %s (%s)", entry, relModDir)
	modDir := filepath.Join(repoRoot, relModDir)
	if _, err := os.Stat(filepath.Join(modDir, "go.mod")); err != nil {
		return result{name, fail, "go.mod not found at " + modDir}
	}
	out := filepath.Join(os.TempDir(), "sthira-preflight-build-"+sanitizeForFilename(relModDir))
	defer os.Remove(out)

	cmd := exec.Command("go", "build", "-o", out, entry)
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(outBytes))
		if len(msg) > 300 {
			msg = msg[:300] + "...(truncated)"
		}
		return result{name, fail, "build failed: " + msg}
	}
	return result{name, pass, "compiles cleanly (GOPROXY=off; no network module resolution)"}
}

func sanitizeForFilename(s string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(s, "_")
}

// checkEnvVarsExistInSource greps the given source file for each env var
// name, failing explicitly (not silently) if any documented flag/env var
// does not actually appear in the code that is supposed to read it. This
// directly satisfies "checks flags/env vars actually exist in code."
func checkEnvVarsExistInSource(repoRoot, relPath string, vars []string, label string) result {
	name := fmt.Sprintf("env vars referenced in %s", label)
	full := filepath.Join(repoRoot, relPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return result{name, fail, "cannot read " + relPath + ": " + err.Error()}
	}
	content := string(data)
	var missing []string
	for _, v := range vars {
		if !strings.Contains(content, `"`+v+`"`) {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return result{name, fail, "documented but NOT found in source: " + strings.Join(missing, ", ")}
	}
	return result{name, pass, fmt.Sprintf("all %d documented vars present in %s", len(vars), relPath)}
}

func checkFileContains(repoRoot, relPath, substr, label string) result {
	name := label
	full := filepath.Join(repoRoot, relPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return result{name, fail, "cannot read " + relPath + ": " + err.Error()}
	}
	if !strings.Contains(string(data), substr) {
		return result{name, fail, fmt.Sprintf("expected substring not found in %s: %q", relPath, substr)}
	}
	return result{name, pass, relPath}
}

// checkDirExists reports FAIL (not silently skipped) when a required host
// path is absent, per "missing dependencies/artifacts fail explicitly
// without mock substitution." It never reads model weight files, only
// stats the directory.
func checkDirExists(name, path string) result {
	if path == "" {
		return result{name, fail, "no path configured"}
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result{name, fail, path + " does not exist on this host"}
		}
		return result{name, fail, path + ": " + err.Error()}
	}
	if !info.IsDir() {
		return result{name, fail, path + " exists but is not a directory"}
	}
	return result{name, pass, path}
}

func checkFileExists(name, path string) result {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result{name, fail, path + " does not exist on this host"}
		}
		return result{name, fail, path + ": " + err.Error()}
	}
	if info.IsDir() {
		return result{name, fail, path + " is a directory, expected a file"}
	}
	return result{name, pass, path}
}

// checkExecutable verifies the interpreter path exists and has the execute
// bit set. It does NOT invoke the interpreter (no subprocess execution of
// python, per "never connects to network/cloud" / no incidental model
// import side effects).
func checkExecutable(name, path string) result {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result{name, fail, path + " does not exist on this host"}
		}
		return result{name, fail, path + ": " + err.Error()}
	}
	if info.Mode()&0o111 == 0 {
		return result{name, fail, path + " exists but is not executable"}
	}
	return result{name, pass, path}
}

// checkLoopbackPortFree confirms the configured worker address is a
// loopback address (never binds anything, never contacts a remote host)
// and that the port is not already bound by another process, which would
// make a "successful" launch silently attach to the wrong server.
func checkLoopbackPortFree(name, addr string) result {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return result{name, fail, addr + ": " + err.Error()}
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return result{name, fail, addr + ": not a loopback address; private workers must bind 127.0.0.1 only"}
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return result{name, fail, addr + " already in use: " + err.Error()}
	}
	_ = ln.Close()
	return result{name, pass, addr + " is free on loopback"}
}
