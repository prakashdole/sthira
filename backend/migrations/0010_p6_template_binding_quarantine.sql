-- B01 strict speech/source/template authorization.
-- Quarantine incomplete active approval rows: a NULL language, source_id
-- or template_sha256 cannot authorize every language/source/template.
-- Re-approval must record the full exact binding. Historical revoked rows
-- may retain NULL fields for audit.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (10)
ON CONFLICT (revision) DO NOTHING;

UPDATE approved_translations
SET revoked_at = GREATEST(approved_at, now())
WHERE revoked_at IS NULL
  AND (language IS NULL OR source_id IS NULL OR template_sha256 IS NULL);

ALTER TABLE approved_translations
    DROP CONSTRAINT IF EXISTS approved_translations_active_complete;

ALTER TABLE approved_translations
    ADD CONSTRAINT approved_translations_active_complete
    CHECK (
        revoked_at IS NOT NULL
        OR (
            language IS NOT NULL
            AND source_id IS NOT NULL
            AND template_sha256 IS NOT NULL
        )
    );

COMMIT;
