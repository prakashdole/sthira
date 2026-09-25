# Hardware and artifact blocker — p6-middle

2026-09-21 · recorded at the contract-freeze / Sarvam-30B adoption state. **Real inference evaluation is BLOCKED** until every item below is addressed and the recorded evidence is reviewed.

## User-selected middle candidate

| Field | Value |
| --- | --- |
| Model | `sarvamai/sarvam-30b` (user-selected, superseding earlier Qwen candidate) |
| Architecture | Mixture-of-Experts (MoE): 128 experts, top-6 routed |
| Parameters | 30B total, 2.4B active non-embedding parameters |
| Precision/Quantization | FP8 weights selected (~30 GB resident weight memory) |
| License | Apache-2.0 (verified from upstream model card) |
| Thinking mode | disabled (`enable_thinking=false` in chat template; non-thinking JSON output) |
| Trust remote code | **true** (required for Sarvam-30B architecture in transformers/vLLM) |
| vLLM / SGLang image tag | **NOT_EVALUATED** (vLLM PR #33942 / fork / hotpatch or SGLang) |
| FP8 artifact SHA-256 | **NOT_EVALUATED** |
| BF16 artifact SHA-256 | **NOT_EVALUATED** |

## Blocked items (must be addressed before benchmark)

1. **GPU hardware.** Sarvam-30B in FP8 requires ~30 GB resident weight memory alone, plus scales, KV cache, activations, workspace and concurrency overhead (recommend 48 GB+ or 2x 24 GB / 1x 80 GB A100/H100). The approval scope (O04, O08) does not authorize GPU rental. Active parameter count (2.4B) determines compute/FLOPs per token, NOT resident memory.
2. **Artifact inventory.** FP8 (and BF16 reference) weight snapshots must be downloaded, hashed, and recorded here. No tag, digest or path may be invented.
3. **vLLM/SGLang runtime support.** The runtime must support Sarvam-30B MoE (via vLLM PR #33942, custom fork, hotpatch, or SGLang) and guided JSON-schema generation (source register S02).
4. **License + remote-code review.** Upstream declares Apache-2.0 and requires `trust_remote_code=true`.
5. **Reviewer sign-off.** Per O15, real-model evaluation must be approved before any reviewer-led run.

## Reproducible commands (when artifacts and GPU are present)

```sh
# 1. Pin the runtime image and start the private server (loopback only).
docker run --gpus all --network=host \
  vllm/vllm-openai:<PINNED_TAG> \
  --model sarvamai/sarvam-30b \
  --trust-remote-code true \
  --quantization fp8 \
  --guided-decoding-backend lm-format-enforcer \
  --max-model-len 4096 \
  --max-num-seqs 2 \
  --port 8000

# 2. Run FP8 evaluation against the reviewed synthetic corpus.
go run ./backend/internal/middleworker/eval \
  -mode benchmark \
  -quantization fp8 \
  -endpoint http://127.0.0.1:8000 \
  -corpus eval/corpus/synthetic.jsonl \
  -out eval/results/bf16.json

# 3. Run AWQ-int4 evaluation on the same reviewed corpus.
go run ./backend/internal/middleworker/eval \
  -mode benchmark \
  -quantization awq-int4 \
  -endpoint http://127.0.0.1:8000 \
  -corpus eval/corpus/synthetic.jsonl \
  -out eval/results/awq.json

# 4. Compare the two result files; report any measured regressions.
```

## Honest evidence boundary

Until the items above are resolved, every claim about real-model
behaviour is **NOT_EVALUATED**. The eval driver supports a
`harness-check` mode that exercises the offline harness against
the stub runtime; that is the only verifiable evidence today.
Tests cover timeout, cancellation, oversized/malformed/extra-text
responses, schema-unsupported, and prompt-injection-shaped input
against a fake HTTP transport. They are NOT proof of model
behaviour; they are proof that the adapter fails closed.

## Concurrency claim discipline

The worker is bounded by `QueueDepth` and `MaxInFlight` set at
construction. Concurrency claims MUST be measured on the
deployed hardware with the chosen quantization. Weight-size
arithmetic is not a throughput estimate. Sustained throughput
will be measured in P7 (worker 11) against realistic traffic
mixes, not extrapolated from `4B × FP16 = 8 GB`.
