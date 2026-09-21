// Sarvam-30B MoE adapter configuration for the private vLLM endpoint.
//
// This file defines the deployment-time parameters for serving
// sarvamai/sarvam-30b through vLLM. The Go middleware worker does NOT
// load the model; it calls the vLLM HTTP API. This configuration
// drives:
//   - The model_id sent in every /v1/chat/completions request
//   - The structured-output schema enforcement
//   - The chat template reasoning controls
//   - The runtime startup commands
//
// Decision D59: User selected Sarvam-30B FP8 MoE. Total weights ~30 GB
// in FP8 quantization. 2.4B non-embedding active parameters from 30B
// total (Mixture of Experts). This file records the deployment config;
// it does NOT prove performance or language accuracy.
//
// NOT_RUN evidence: Hardware, GPU memory, and vLLM compatibility are
// not verified here. The config is structurally correct per the
// Sarvam-30B model card and vLLM documentation. Actual inference
// requires designated GPU infrastructure.

package middleworker

import "time"

// SarvamModelID is the model identifier sent to vLLM.
const SarvamModelID = "sarvamai/sarvam-30b"

// SarvamConfig returns a production HTTPClientRuntimeConfig for
// Sarvam-30B FP8 on vLLM/SGLang.
//
// Key parameters:
//   - Temperature 0.0: deterministic output (no sampling variance)
//   - max_tokens 256: bounded output per the action contract
//   - Structured output via response_format json_schema (the shape
//     in the pinned vLLM structured-outputs docs, fetched 2026-09-21)
//   - enable_thinking=false via chat_template_kwargs: the OFFICIAL
//     Sarvam chat_template.jinja (raw file fetched from the model
//     repo 2026-09-21) consumes the “enable_thinking“ Jinja
//     variable and appends the model's “<|nothink|>“ token to the
//     last user turn when it is defined and false. We therefore do
//     NOT ship a speculative local template: the template is the
//     one the server loads from the model repo, and our request
//     only sets the variable that template already reads. A
//     previous constant in this file set the variable inside an
//     invented template that never referenced it — a dead switch.
//
// Deployment note (NOT_RUN): the exact vLLM release the serving
// stack pins, and therefore the exact chat-template semantics,
// must be re-verified at deployment; until then this file records
// the request/flag contract, and the b4 test asserts the ACTUAL
// outbound body shape (chat_template_kwargs present, no invented
// per-request chat_template field).
func SarvamConfig(client *Client, system string) HTTPClientRuntimeConfig {
	return HTTPClientRuntimeConfig{
		Client:             client,
		ModelID:            SarvamModelID,
		Revision:           "", // populated by /health from vLLM
		DigestName:         "sarvamai/sarvam-30b",
		DigestSHA:          "", // populated by artifact verification
		Languages:          SarvamSupportedLanguages(),
		System:             system,
		ChatTemplateKwargs: SarvamChatTemplateKwargs(),
	}
}

// SarvamChatTemplateKwargs are the Jinja variables this client sends
// with every request to the server-loaded official template.
// enable_thinking=false suppresses internal reasoning tokens so
// they cannot consume the bounded output budget or leak into the
// JSON action channel.
func SarvamChatTemplateKwargs() map[string]any {
	return map[string]any{"enable_thinking": false}
}

// SarvamSupportedLanguages returns the language codes Sarvam-30B
// claims to support per its model card. The worker advertises only
// the intersection of these with verified evaluation evidence.
// Currently the evaluation harness has tested hi-IN, ml-IN, en-IN.
func SarvamSupportedLanguages() []string {
	return []string{
		"en-IN", // English (India)
		"hi-IN", // Hindi
		"ml-IN", // Malayalam
	}
}

// SarvamLimits returns client limits tuned for Sarvam-30B FP8.
// The 30B MoE model has higher latency than dense 4B models due to
// expert routing; we increase the per-call timeout accordingly.
func SarvamLimits() Limits {
	return Limits{
		MaxContextTokens: 4096,
		MaxOutputTokens:  256,
		MaxResponseBytes: 64 * 1024,
		MaxRequestBytes:  64 * 1024,
		ConnectTimeout:   3 * time.Second,
		PerCallTimeout:   8 * time.Second, // higher than 4B model's 6s
	}
}

// SarvamVLLMStartupArgs returns the recommended vLLM CLI arguments
// for serving Sarvam-30B FP8. These are recorded for operator
// reference; the Go worker does NOT start vLLM.
//
// Key flags:
//   - --model sarvamai/sarvam-30b: the HF model identifier
//   - --quantization fp8: FP8 quantization (~30 GB weights)
//   - --dtype auto: let vLLM select the compute dtype
//   - --max-model-len 4096: matches MaxContextTokens
//   - --gpu-memory-utilization 0.90: leave headroom for KV cache
//   - --enforce-eager: disable CUDA graphs for MoE compatibility
//   - --enable-prefix-caching: share system prompt KV across requests
//   - --structured-outputs-config.backend: request-level json_schema
//     enforcement; vLLM >=0.12 removed the --guided-decoding-backend
//     flag and the guided_* request fields (verified against the
//     pinned structured-outputs docs, fetched 2026-09-21). Default
//     "auto" is left in place here; pin a backend only after the
//     deployment's vLLM version is recorded.
//   - --host 127.0.0.1 --port 8000: private loopback only
//
// The official chat_template.jinja ships inside the model repo and
// is loaded by vLLM automatically; no --chat-template override is
// passed (and none is sent per-request — see SarvamChatTemplateKwargs
// for how reasoning is controlled through the template's own
// documented variable). FP8 flags target Ada/Hopper GPUs; hardware
// compatibility is NOT_RUN evidence here.
func SarvamVLLMStartupArgs() []string {
	return []string{
		"--model", SarvamModelID,
		"--quantization", "fp8",
		"--dtype", "auto",
		"--max-model-len", "4096",
		"--gpu-memory-utilization", "0.90",
		"--enforce-eager",
		"--enable-prefix-caching",
		"--host", "127.0.0.1",
		"--port", "8000",
	}
}

// SarvamWeightsResidencyBytes documents the FP8 weight footprint
// (≈30 GB) as DISTINCT from the 2.4B active non-embedding
// parameters per MoE token. Resident bytes are a deployment
// property; active compute is a per-token property; neither is a
// measurement of throughput or accuracy (NOT_RUN until designated
// hardware). Kept as documentation so no code path can conflate
// "30B resident" with "2.4B per token".
const SarvamWeightsResidencyBytes = 30 * 1000 * 1000 * 1000
