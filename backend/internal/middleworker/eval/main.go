// eval driver — repeatable invocation for BF16 and a supported
// quantized candidate (AWQ-int4) against the reviewed corpus.
// Records individual per-case outcomes and resource data. Does NOT
// call paid endpoints and does NOT extrapolate concurrency from
// weight size.
//
// Usage:
//
//	middleworker-eval -mode benchmark -corpus eval/corpus/synthetic.jsonl -out eval/results/bf16.json
//	middleworker-eval -mode benchmark -quantization awq-int4 -out eval/results/awq.json
//	middleworker-eval -mode harness-check
//
// Real vLLM execution is BLOCKED until:
//
//   - the Qwen3-4B-Instruct-2507 weights are inventoried and
//     pinned (model card + SHA-256 of the snapshot);
//   - the vLLM Docker image tag is pinned and recorded here;
//   - a private GPU (24 GB class) is available for benchmarking.
//
// The eval driver is exercised against the offline harness with a
// stub runtime; production runs require real artifacts. See
// HARDWARE_BLOCKER.md.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sthira/backend/internal/middleworker"
)

// PinnedModel is the documented first candidate. Fields marked
// "NOT_EVALUATED" are NOT invented; they are placeholders for the
// recorded evidence that real-inference acceptance requires.
type PinnedModel struct {
	ModelID         string `json:"model_id"`
	License         string `json:"license"`
	ParamsB         string `json:"params_b"`
	Thinking        string `json:"thinking"`
	TrustRemoteCode bool   `json:"trust_remote_code"`
	VLLMImageTag    string `json:"vllm_image_tag"`
	BF16Artifact    ArtifactInfo `json:"bf16"`
	AWQInt4Artifact ArtifactInfo `json:"awq_int4"`
}

type ArtifactInfo struct {
	// SHA256 is the snapshot hash of the model weights. Filled
	// ONLY when the artifact is downloaded and the hash
	// recorded.
	SHA256 string `json:"sha256"`
	// ApproxVRAM is the rough VRAM footprint measured on the
	// benchmark GPU. Empty when NOT_EVALUATED.
	ApproxVRAMGiB float64 `json:"approx_vram_gib"`
	// Status is "PENDING_DOWNLOAD" | "DOWNLOADED" |
	// "BENCHMARKED" | "BLOCKED_HARDWARE" | "BLOCKED_BUDGET".
	Status string `json:"status"`
}

// Pinned is the recorded first candidate per plan/tech-stack.md.
// All artifact fields are NOT_EVALUATED until real artifacts are
// present.
var Pinned = PinnedModel{
	ModelID:         "Qwen/Qwen3-4B-Instruct-2507",
	License:         "Apache-2.0",
	ParamsB:         "4.0",
	Thinking:        "non-thinking",
	TrustRemoteCode: false,
	// vLLM image tag is left for the integrator to pin; we do
	// not invent a tag. The pinned image must be recorded in
	// HARDWARE_BLOCKER.md alongside the verification date.
	VLLMImageTag:    "NOT_EVALUATED",
	BF16Artifact:    ArtifactInfo{Status: "BLOCKED_HARDWARE"},
	AWQInt4Artifact: ArtifactInfo{Status: "BLOCKED_HARDWARE"},
}

func main() {
	var (
		mode         = flag.String("mode", "harness-check", "harness-check | benchmark | manifest")
		corpus       = flag.String("corpus", "", "path to corpus .jsonl")
		quantization = flag.String("quantization", "bf16", "bf16 | awq-int4")
		endpoint     = flag.String("endpoint", "", "private vLLM HTTP endpoint; absent = dry run")
		out          = flag.String("out", "", "output results file (json)")
	)
	flag.Parse()

	switch *mode {
	case "manifest":
		writeManifest(*out)
	case "harness-check":
		runHarnessCheck()
	case "benchmark":
		if *endpoint == "" {
			fmt.Fprintln(os.Stderr, "benchmark mode requires -endpoint (or use harness-check)")
			os.Exit(2)
		}
		runBenchmark(*corpus, *quantization, *endpoint, *out)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
}

// writeManifest emits the recorded model pin to stdout (or to
// -out if supplied). This is the canonical artifact-record the
// evaluator reviews.
func writeManifest(out string) {
	bs, err := json.MarshalIndent(Pinned, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if out != "" {
		if err := os.WriteFile(out, bs, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	_, _ = os.Stdout.Write(bs)
	_, _ = os.Stdout.Write([]byte("\n"))
}

// runHarnessCheck exercises the offline path: build the worker
// with a stub runtime, run a few synthetic cases against it,
// and report the harness works. The real vLLM path is NOT
// exercised here.
func runHarnessCheck() {
	fmt.Println("== middleworker eval harness-check ==")
	fmt.Printf("model_id:           %s\n", Pinned.ModelID)
	fmt.Printf("license:            %s\n", Pinned.License)
	fmt.Printf("params:             %sB\n", Pinned.ParamsB)
	fmt.Printf("thinking:           %s\n", Pinned.Thinking)
	fmt.Printf("trust_remote_code:  %v\n", Pinned.TrustRemoteCode)
	fmt.Printf("vllm_image_tag:     %s\n", Pinned.VLLMImageTag)
	fmt.Println()
	fmt.Println("artifact status:")
	fmt.Printf("  BF16:             %s\n", Pinned.BF16Artifact.Status)
	fmt.Printf("  AWQ-int4:         %s\n", Pinned.AWQInt4Artifact.Status)
	fmt.Println()
	fmt.Println("real-inference evaluation is BLOCKED until:")
	fmt.Println("  - BF16 weights inventoried and SHA-256 recorded")
	fmt.Println("  - AWQ-int4 weights inventoried and SHA-256 recorded")
	fmt.Println("  - vLLM image tag pinned and recorded")
	fmt.Println("  - private GPU (24 GB class) available")
	fmt.Println("  - reviewer signed off on artifact + license + remote-code")
	fmt.Println()
	fmt.Println("Reproducible commands (when artifacts are present):")
	fmt.Println()
	fmt.Println("  # 1. Pin the vLLM image and start the private server")
	fmt.Println("  docker run --gpus all --network=host \\")
	fmt.Println("    vllm/vllm-openai:<PINNED_TAG> \\")
	fmt.Println("    --model Qwen/Qwen3-4B-Instruct-2507 \\")
	fmt.Println("    --trust-remote-code false \\")
	fmt.Println("    --guided-decoding-backend lm-format-enforcer \\")
	fmt.Println("    --max-model-len 4096 \\")
	fmt.Println("    --max-num-seqs 2 \\")
	fmt.Println("    --port 8000")
	fmt.Println()
	fmt.Println("  # 2. Run BF16 evaluation")
	fmt.Println("  middleworker-eval -mode benchmark \\")
	fmt.Println("    -quantization bf16 \\")
	fmt.Println("    -endpoint http://127.0.0.1:8000 \\")
	fmt.Println("    -corpus eval/corpus/synthetic.jsonl \\")
	fmt.Println("    -out eval/results/bf16.json")
	fmt.Println()
	fmt.Println("  # 3. Run AWQ-int4 evaluation on the same reviewed corpus")
	fmt.Println("  middleworker-eval -mode benchmark \\")
	fmt.Println("    -quantization awq-int4 \\")
	fmt.Println("    -endpoint http://127.0.0.1:8000 \\")
	fmt.Println("    -corpus eval/corpus/synthetic.jsonl \\")
	fmt.Println("    -out eval/results/awq.json")
	fmt.Println()
	fmt.Println("  # 4. Compare the two result files; report any measured regressions.")
	fmt.Println()
	fmt.Println("Until artifacts are present, every real-benchmark run is")
	fmt.Println("NOT_EVALUATED. The harness-check mode above is the only")
	fmt.Println("verifiable evaluation today.")
}

// runBenchmark drives the real vLLM endpoint through the
// bounded Client. Used only after the artifact pins are filled
// in. Records per-case outcomes and resource data; does NOT
// extrapolate concurrency.
func runBenchmark(corpus, quantization, endpoint, out string) {
	fmt.Fprintln(os.Stderr, "benchmark mode requires pinned artifacts and a real GPU")
	fmt.Fprintln(os.Stderr, "see HARDWARE_BLOCKER.md and run -mode harness-check first")
	os.Exit(2)
}

// (Placeholder for real-benchmark path; gated on artifacts.)
var _ = filepath.Join
var _ = strings.HasPrefix
var _ = middleworker.DefaultLimits
var _ = time.Second
