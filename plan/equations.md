# Controlled calculations and sizing

2026-09-19. No hazard, red-zone, land suitability or route-safety equation belongs in this product.

## Capacity conservation

For a facility (and each relevant date/slot for temporary stays):

```text
E = official_capacity + authorized_adjustments
E = free + held + occupied
free >= 0; held >= 0; occupied >= 0
reserve(p): free -= p; held += p
arrive(p): held -= p; occupied += p
expire_or_cancel_hold(p): held -= p; free += p
depart(p): occupied -= p; free += p
```

Arrival after a reservation does not subtract free space again. A walk-in requires its own authorized transaction/policy. NO/dismiss/network timeout is not arrival. Official reductions below committed occupancy generate a controlled conflict/escalation, not negative free space or erased residents. Temporary date ranges must be available across the whole requested interval. A transfer cannot free old occupancy merely because a new request was sent.

Transactions persist the event, scoped idempotency key, payload hash, result and outbox/audit together. Duplicate same-payload requests return the stored result; changed payloads conflict. These semantics must hold across processes and restarts, not only one language lock.

## Eligibility and citizen choice

```text
eligible = authorized_active_facility
           AND current_incident_and_policy
           AND party_and_stay_requirements_supported
           AND capacity_policy_satisfied
           AND operational_route_gate_satisfied
```

Until route authority is resolved, the operational route gate is false. Synthetic policies can exercise it in demo isolation. Eligible choices may be sorted by supplied route length when the authority policy permits. No straight-line distance, model preference, road surface guess or safety score establishes eligibility. Unknown capacity can be displayed as unknown in informational mode; it cannot become a reservation promise.

## Freshness

```text
valid_until = min(applicable explicit expiries,
                  issued_or_observed_time + authorized_max_age)
current = authorized AND validated AND effective_from <= now < valid_until
          AND not superseded AND not revoked AND clock_is_sufficiently_trusted
```

Handle missing expiry only with an approved maximum-age rule. Test future timestamps, timezone offsets, clock rollback, cancellation received before its target and replay of an old manifest. Stay dates are independent of route validity.

## Load example — assumptions from parameters.md

```text
public_request_rate = active_users × reads_per_user_second
origin_public_reads = public_request_rate × (1 - public_cache_hit_rate)
write_rate = active_users × writes_per_user_second
voice_arrival_rate = active_users × utterances_per_user_second
```

| Active sessions | Public reads/s before cache | Origin reads/s at 95% hit | Writes/s | Voice utterances/s |
| --- | --- | --- | --- | --- |
| 10,000 | 1,000 | 50 | 20 | 20 |
| 50,000 | 5,000 | 250 | 100 | 100 |
| 100,000 | 10,000 | 500 | 200 | 200 |

These are workload scenarios, not measured capacity. A cache outage sends the full public rate to the origin; design overload behavior and test it. Private writes are not reduced by public CDN hit rate. Short synchronized arrival bursts matter more than daily averages.

## Inference sizing

```text
average_inflight ≈ arrival_rate × mean_service_seconds
replicas >= ceil(peak_arrival_rate / measured_sustainable_rate_per_replica)
```

Use sustainable throughput at the required p95/p99 latency, not maximum token throughput. Then add failure reserve/headroom and validate under burst/queue behavior. Size ASR, middle model and TTS independently. TTS demand can be lower through approved audio reuse; ASR demand cannot be inferred from LLM tokens.

```text
GPU_memory ≈ weights + KV_cache(active_sequences, context) + runtime_workspace
weight_bytes ≈ parameters × bits_per_weight / 8
```

For 4B: BF16 weights ~8 billion bytes; ideal 4-bit weights ~2 billion bytes. Actual memory is higher. Quantization changes need rerun language/action evaluations. Long context can overwhelm memory even with small weights.

```text
voice_latency = upload + ASR_queue/compute + middle_queue/compute
                + validation + response_download + client_apply
```

TTS time is additional only when speech is needed; deliver validated visual/text results without waiting for it. Record cold/warm behavior and network contributions separately.

## Bandwidth and money

```text
transfer_seconds_lower_bound = payload_bytes × 8 / usable_bits_per_second
monthly_cost = API/worker_compute + DB/backup + GPU_hours
               + storage + CDN/egress + monitoring + operations
cost_per_successful_voice_task = attributable_cost / successful_tasks
```

A 64 KiB response takes about 1.31 s at 400 kbit/s before RTT/loss/protocol overhead. Cached assets avoid repeated transfer; compression cannot make a cold multi-megabyte map instant. State cost for normal and disaster-surge months separately. No bill/server count is promised without actual provider prices, hardware benchmarks and a budget.
