-- P4 operator grants: server-controlled authorization for operator issuance.
-- Additive only.
--
-- An operator session is issued only when a trusted identity/MFA boundary has
-- verified a principal AND a server-controlled grant binds that verified
-- subject to the jurisdiction. Grants are provisioned out-of-band (never via a
-- public request); issuance checks them and fails closed when none matches.
-- This replaces trusting a request-body mfa_verified flag or requested
-- jurisdiction, which let a caller self-grant operator authority.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (4)
ON CONFLICT (revision) DO NOTHING;

CREATE TABLE operator_grants (
    grant_id      text PRIMARY KEY,
    -- subject binds the grant to the verified identity the trusted boundary
    -- attested (e.g. an IdP subject claim). Never a caller-chosen label.
    subject       text NOT NULL,
    jurisdiction  text NOT NULL,
    granted_by    text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz,               -- NULL = no expiry
    revoked_at    timestamptz,               -- set = grant withdrawn
    UNIQUE (subject, jurisdiction)
);

COMMIT;

-- DOWN:
-- BEGIN;
-- DROP TABLE IF EXISTS operator_grants;
-- DELETE FROM schema_migrations WHERE revision = 4;
-- COMMIT;
