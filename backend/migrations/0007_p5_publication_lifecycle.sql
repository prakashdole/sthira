-- P5 publication lifecycle: add source_id and jurisdiction attribution to
-- published manifests and cards, backfilling from the packages table where
-- possible, supporting authority propagation (quarantine/withdrawal) and
-- explicit staged-to-current promotion.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (7)
ON CONFLICT (revision) DO NOTHING;

ALTER TABLE published_manifests ADD COLUMN IF NOT EXISTS source_id text;
ALTER TABLE published_cards ADD COLUMN IF NOT EXISTS source_id text;
ALTER TABLE published_cards ADD COLUMN IF NOT EXISTS jurisdiction text;

-- Backfill attribution from packages where available
UPDATE published_manifests pm
SET source_id = p.source_id
FROM packages p
WHERE pm.package_id = p.package_id AND pm.source_id IS NULL;

UPDATE published_cards pc
SET source_id = p.source_id,
    jurisdiction = p.jurisdiction
FROM packages p
WHERE pc.package_id = p.package_id AND (pc.source_id IS NULL OR pc.jurisdiction IS NULL);

CREATE INDEX IF NOT EXISTS published_manifests_source_idx ON published_manifests (source_id);
CREATE INDEX IF NOT EXISTS published_cards_source_idx ON published_cards (source_id);
CREATE INDEX IF NOT EXISTS published_cards_jurisdiction_idx ON published_cards (jurisdiction);

COMMIT;
