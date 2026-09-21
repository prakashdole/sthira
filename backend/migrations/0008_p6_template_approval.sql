-- P6 template approval: persisted, jurisdiction-bound approval of the
-- speech_keys a citizen may actually be served. The trusted ScopedContext
-- exposes these as TemplateKeys; the ProductionValidator rejects any
-- proposal whose speech_key is not in that set (contracts/scoped.go).
--
-- Production stays fail-closed until a translation authority records an
-- approval here (external gate O11): an empty table means NO speech is
-- approved, and the pipeline will not synthesise unreviewed text. This is
-- the boundary Worker A's "empty TemplateKeys is authoritative" rule needs
-- a real source of truth for.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (8)
ON CONFLICT (revision) DO NOTHING;

CREATE TABLE IF NOT EXISTS approved_translations (
    translation_id   text PRIMARY KEY,
    jurisdiction     text NOT NULL,
    speech_key       text NOT NULL,
    -- NULL language = approved for every language the jurisdiction serves;
    -- a concrete language restricts the approval to that reviewer sign-off.
    language         text,
    source_version   integer NOT NULL CHECK (source_version >= 1),
    template_version integer NOT NULL CHECK (template_version >= 1),
    approved_by      text NOT NULL,
    evidence_ref     text NOT NULL,
    approved_at      timestamptz NOT NULL,
    revoked_at       timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (jurisdiction, speech_key, language, source_version, template_version),
    CHECK (revoked_at IS NULL OR revoked_at >= approved_at)
);

-- Active (non-revoked) approvals for a jurisdiction are the common lookup.
CREATE INDEX IF NOT EXISTS approved_translations_active_idx
    ON approved_translations (jurisdiction, source_version)
    WHERE revoked_at IS NULL;

COMMIT;
