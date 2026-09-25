-- P6 template binding: binds approved_translations to authoritative source_id
-- and template_sha256 digest, preventing cross-source and cross-template
-- approval leakage.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (9)
ON CONFLICT (revision) DO NOTHING;

ALTER TABLE approved_translations
    ADD COLUMN IF NOT EXISTS source_id text REFERENCES sources(source_id);

ALTER TABLE approved_translations
    ADD COLUMN IF NOT EXISTS template_sha256 text CHECK (template_sha256 IS NULL OR char_length(template_sha256) = 64);

CREATE INDEX IF NOT EXISTS approved_translations_binding_idx
    ON approved_translations (jurisdiction, source_id, source_version, template_version)
    WHERE revoked_at IS NULL;

COMMIT;
