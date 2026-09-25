-- P4 operator access: verified-identity + MFA boundary for operational access.
-- Additive only.
--
-- An OPERATOR session is not stronger authentication by label alone (R22): it
-- requires a verified identity and a second factor. We record when MFA was
-- verified on the session; operational endpoints require it present and recent.
-- Jurisdiction scoping limits an operator to their own jurisdiction.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (3)
ON CONFLICT (revision) DO NOTHING;

-- When the operator's second factor was verified. NULL = MFA not completed;
-- operational access is denied. Issuance sets it from verified MFA evidence.
ALTER TABLE sessions
    ADD COLUMN mfa_verified_at timestamptz;

-- An OPERATOR session must carry MFA verification to be usable for operations.
-- (Citizens never need it.) Enforced in the store/handler, documented here.

COMMIT;

-- DOWN:
-- BEGIN;
-- ALTER TABLE sessions DROP COLUMN IF EXISTS mfa_verified_at;
-- DELETE FROM schema_migrations WHERE revision = 3;
-- COMMIT;
