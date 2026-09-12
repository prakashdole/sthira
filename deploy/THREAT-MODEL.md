# v2 synthetic boundary threat model

| Threat | Local control | Remaining evidence |
| --- | --- | --- |
| XXE/entity expansion | CAP parser rejects DTD/entity declarations and bounds XML size | Fuzz and production gateway review |
| Imported HTML/script | Imported text is escaped before public rendering | Full dynamic/API scan |
| Voice prompt injection | Deterministic grammar and ID-only map-action validator; prohibited requests have zero actions | Model red-team corpus and inference DoS benchmark |
| Replay/idempotency | Assignment and arrival keys are validated and repeated mutations are idempotent | Database uniqueness/locking proof |
| Cross-jurisdiction publication | Package service checks authenticated operator jurisdiction | Real identity/RBAC integration |
| Stale/cancelled guidance | CAP lifecycle and offline cache expiry/invalidation fail closed | PostgreSQL recovery rehearsal |
| Privacy leakage | Raw voice deletion, session expiry, bounded identifiers, no location in ordinary v2 response logs | DPIA, retention job, and legal review |
| Basemap failure | Map error leaves local overlays/text controls available | Browser/platform matrix |

No live source, government credential, personal data, or external model artifact is
used by the synthetic profile.
