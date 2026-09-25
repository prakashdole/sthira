-- P4 operator identity binding: persist the verified identity subject and the
-- authorizing grant on each operator session, and invalidate legacy operator
-- sessions whose trusted identity was never established. Additive only.
--
-- Before this migration an OPERATOR session recorded only principal_kind and
-- jurisdiction; the verified subject existed only transiently at issuance. That
-- made audit attribution stop at the session and let a session outlive the
-- grant that authorized it. Now the session carries the verified subject and
-- the grant it was issued under, so every protected operation can re-check the
-- CURRENT grant (expiry/revocation/jurisdiction) and audit can trace
-- event -> session -> verified identity.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (5)
ON CONFLICT (revision) DO NOTHING;

-- The verified identity subject (e.g. an IdP subject claim) this session was
-- issued for, and the grant that authorized it. NULL for citizen sessions and
-- for legacy operator sessions issued before trusted identity was persisted.
ALTER TABLE sessions
    ADD COLUMN operator_subject  text,
    ADD COLUMN operator_grant_id text REFERENCES operator_grants(grant_id);

-- Invalidate legacy operator sessions whose trusted identity cannot be
-- established. A session minted from caller self-attestation (no verified
-- subject persisted) must never be treated as trusted after this migration;
-- never infer a trusted identity for it. Revoke them so they cannot act.
UPDATE sessions
SET revoked_at = now()
WHERE principal_kind = 'OPERATOR' AND operator_subject IS NULL;

-- From here on, an operator session must carry its verified subject. (Citizens
-- are unaffected.) Enforced at issuance in the store; documented here.
-- Not added as a hard CHECK only because legacy rows are revoked, not deleted;
-- new issuance always sets operator_subject.

COMMIT;

-- DOWN:
-- BEGIN;
-- ALTER TABLE sessions DROP COLUMN IF EXISTS operator_subject,
--     DROP COLUMN IF EXISTS operator_grant_id;
-- DELETE FROM schema_migrations WHERE revision = 5;
-- COMMIT;  (revoked legacy sessions stay revoked; revocation is not reversed)
