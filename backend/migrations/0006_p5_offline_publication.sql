-- P5 offline publication persistence: tables supporting signed regional manifests,
-- immutable public incident cards, and content-addressed auxiliary assets.

BEGIN;

INSERT INTO schema_migrations (revision) VALUES (6)
ON CONFLICT (revision) DO NOTHING;

CREATE TABLE published_manifests (
    manifest_id     text PRIMARY KEY,
    jurisdiction    text NOT NULL,
    revision        integer NOT NULL CHECK (revision >= 1),
    package_id      text NOT NULL,
    raw_json        bytea NOT NULL,
    checksum_sha256 text NOT NULL CHECK (char_length(checksum_sha256) = 64),
    source_status   text NOT NULL DEFAULT 'CURRENT',
    quarantined     boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (jurisdiction, revision)
);

CREATE INDEX published_manifests_jurisdiction_rev_idx 
    ON published_manifests (jurisdiction, revision DESC);

CREATE TABLE published_cards (
    package_id      text NOT NULL,
    version         integer NOT NULL CHECK (version >= 1),
    raw_json        bytea NOT NULL,
    checksum_sha256 text NOT NULL CHECK (char_length(checksum_sha256) = 64),
    source_status   text NOT NULL DEFAULT 'CURRENT',
    quarantined     boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (package_id, version)
);

CREATE TABLE published_resources (
    resource_id     text PRIMARY KEY,
    content_type    text NOT NULL,
    content_length  bigint NOT NULL CHECK (content_length >= 0),
    checksum_sha256 text NOT NULL CHECK (char_length(checksum_sha256) = 64),
    content         bytea NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

COMMIT;

-- DOWN:
-- BEGIN;
-- DROP TABLE IF EXISTS published_resources;
-- DROP TABLE IF EXISTS published_cards;
-- DROP TABLE IF EXISTS published_manifests;
-- DELETE FROM schema_migrations WHERE revision = 6;
-- COMMIT;
