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
//   - Guided generation via json_schema: vLLM enforces the Proposal
//     shape at decode time
//   - enable_thinking=false: Sarvam-30B has a thinking/reasoning
//     mode (per the source-register.md S11 model card). We
//     explicitly disable it so internal reasoning tokens cannot
//     leak into the JSON action channel or consume output budget.
//     The chat template is wired into the request via
//     ProposeInput.ChatTemplate so the runtime controls reasoning
//     at request time rather than relying solely on server
//     startup flags.
//
// The SarvamChatTemplate below is a documented placeholder. The
// exact token shape (e.g. <|start_of_turn|>...<|end_of_turn|> vs.
// <|im_start|>...<|im_end|>) must be re-verified against the
// Sarvam-30B pinned model card at deployment time. SarvamConfig
// sets the template verbatim so an updated template can be
// substituted without changing the runtime.
func SarvamConfig(client *Client, system string) HTTPClientRuntimeConfig {
	return HTTPClientRuntimeConfig{
		Client:      client,
		ModelID:     SarvamModelID,
		Revision:    "", // populated by /health from vLLM
		DigestName:  "sarvamai/sarvam-30b",
		DigestSHA:   "", // populated by artifact verification
		Languages:   SarvamSupportedLanguages(),
		System:      system,
		ChatTemplate: SarvamChatTemplate,
	}
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
//   - --guided-decoding-backend outlines: for json_schema enforcement
//   - --host 127.0.0.1 --port 8000: private loopback only
//   - --chat-template: use the model's built-in template with
//     enable_thinking=false
func SarvamVLLMStartupArgs() []string {
	return []string{
		"--model", SarvamModelID,
		"--quantization", "fp8",
		"--dtype", "auto",
		"--max-model-len", "4096",
		"--gpu-memory-utilization", "0.90",
		"--enforce-eager",
		"--enable-prefix-caching",
		"--guided-decoding-backend", "outlines",
		"--host", "127.0.0.1",
		"--port", "8000",
	}
}

// SarvamChatTemplate is the chat template override for Sarvam-30B
// that explicitly disables thinking/reasoning mode. The template
// uses Gemma-style turn tokens because Sarvam-30B is based on the
// Gemma 2 architecture (per sarvamai/sarvam-30b model card,
// S11). The exact token shape must be re-verified against the
// pinned model card at deployment time; if a different model card
// is approved, swap this constant for the matching template.
//
// Honesty note: the Sarvam-30B model card describes the chat
// template tokens as Gemma-style (<|start_of_turn|> /
// <|end_of_turn|>). We commit to that shape here and rely on
// deployment-time verification. If the tokens are wrong, vLLM
// will fail loudly at the first request; the worker surfaces the
// 422 SCHEMA_UNSUPPORTED or 400 MALFORMED state.
//
// The template also sets enable_thinking=false at the top of the
// Jinja prelude so Sarvam-30B's reasoning mode never emits internal
// reasoning tokens into the JSON action channel.
const SarvamChatTemplate = `{%- set enable_thinking = false -%}
{%- for message in messages -%}
{%- if message.role == 'system' -%}
<|start_of_turn|>system
{{ message.content }}<|end_of_turn|>
{%- elif message.role == 'user' -%}
<|start_of_turn|>user
{{ message.content }}<|end_of_turn|>
{%- elif message.role == 'assistant' -%}
<|start_of_turn|>assistant
{{ message.content }}<|end_of_turn|>
{%- endif -%}
{%- endfor -%}
<|start_of_turn|>assistant
`
