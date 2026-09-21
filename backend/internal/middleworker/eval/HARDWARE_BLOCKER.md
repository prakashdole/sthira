# Hardware and artifact blocker — p6-middle

2026-09-21 · recorded by Worker 6 (p6-middle) at the contract-freeze
state. **Real vLLM evaluation is BLOCKED** until every item below is
addressed and the recorded evidence is reviewed.

## Pinned first candidate

| Field | Value |
| --- | --- |
| Model | `Qwen/Qwen3-4B-Instruct-2507` |
| Parameters | 4.0B (verified from model card, source register S01) |
| License | Apache-2.0 (verified) |
| Thinking mode | non-thinking |
| Trust remote code | **false** (explicit; no silent trust) |
| vLLM image tag | **NOT_EVALUATED** (to be pinned during integration) |
| BF16 artifact SHA-256 | **NOT_EVALUATED** |
| AWQ-int4 artifact SHA-256 | **NOT_EVALUATED** |

## Blocked items (must be addressed before benchmark)

1. **GPU hardware.** A 24 GB-class private GPU is required to
   benchmark the 4B model with realistic concurrency. The
   approval scope (O08, O12) does not authorize GPU rental.
   The 4B model at BF16 needs roughly 8 GB just for the weights;
   AWQ-int4 needs roughly 2–3 GB. Neither includes KV cache,
   runtime workspace, quantization overhead, or concurrency.
2. **Artifact inventory.** Both BF16 and AWQ-int4 weight
   snapshots must be downloaded, hashed, and recorded here.
   No tag, digest or path may be invented.
3. **vLLM image tag.** The exact vLLM Docker image tag must
   be pinned and recorded here. The image must support
   guided JSON-schema generation (per source register S02);
   a version that drifts from the documented
   `/v1/chat/completions` schema is a deployment blocker.
4. **License + remote-code review.** The reviewer must confirm
   that the Qwen3-4B-Instruct-2507 weights do NOT require
   remote code; the worker sets `trust_remote_code=false` at
   the API surface.
5. **Reviewer sign-off.** Per O15, real-model evaluation must
   be approved before any reviewer-led run.

## Reproducible commands (when artifacts and GPU are present)

```sh
# 1. Pin the vLLM image and start the private server (loopback only).
docker run --gpus all --network=host \
  vllm/vllm-openai:<PINNED_TAG> \
  --model Qwen/Qwen3-4B-Instruct-2507 \
  --trust-remote-code false \
  --guided-decoding-backend lm-format-enforcer \
  --max-model-len 4096 \
  --max-num-seqs 2 \
  --port 8000

# 2. Run BF16 evaluation against the reviewed synthetic corpus.
go run ./backend/internal/middleworker/eval \
  -mode benchmark \
  -quantization bf16 \
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
