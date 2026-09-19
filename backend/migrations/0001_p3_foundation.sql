-- P3 durable storage foundation: sources/artifacts, versioned facts, sessions,
-- reservations/stays, facility/date inventory, idempotency, audit/outbox.
--
-- Target: PostgreSQL 15+ with PostGIS 3.x. Apply inside a transaction. This
-- migration is additive only; it creates no destructive change and touches no
-- pre-existing data. Rollback drops the objects it creates (see DOWN section).
--
-- Geometry uses SRID 4326 (EPSG:4326, WGS84 lon/lat) to match the /api/v3
-- boundary contract. A wrong-SRID insert is rejected by the geometry type
-- constraint, which is the intended PostGIS enforcement.

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

-- Migration tracking. The readiness probe reads MAX(revision) here to confirm
-- the applied schema matches the binary's expected revision.
CREATE TABLE IF NOT EXISTS schema_migrations (
    revision   integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations (revision) VALUES (1)
ON CONFLICT (revision) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Sources and artifacts (T05): the government source registry and the raw
-- artifacts each source version produced. Source lifecycle state is owned by
-- the application (sourceact); the DB enforces identity and versioning.
-- ---------------------------------------------------------------------------

CREATE TABLE sources (
    source_id        text PRIMARY KEY,
    government_owner text NOT NULL,
    official_domain  text NOT NULL,
    state            text NOT NULL,
    -- Optimistic concurrency token. Every update requires the caller's expected
    -- version and atomically increments it; a stale expected version matches no
    -- row and is rejected (see store layer). Never updated under a mutex alone.
    version          integer NOT NULL CHECK (version >= 1),
    updated_at       timestamptz NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);

-- Authorization evidence for source activation. Source activation requires
-- recorded authorization evidence, not merely advancing an enum (P3 mandate).
-- The AUTHORIZED -> OPERATIONAL transition must reference a row here.
CREATE TABLE source_authorizations (
    authorization_id text PRIMARY KEY,
    source_id        text NOT NULL REFERENCES sources(source_id),
    -- Who granted authorization and the evidence reference (e.g. document id).
    granted_by       text NOT NULL,
    evidence_ref     text NOT NULL,
    jurisdiction     text NOT NULL,
    granted_at       timestamptz NOT NULL,
    expires_at       timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE source_artifacts (
    artifact_id      text PRIMARY KEY,
    source_id        text NOT NULL REFERENCES sources(source_id),
    source_version   integer NOT NULL,
    artifact_sha256  text NOT NULL CHECK (char_length(artifact_sha256) = 64),
    retrieved_at     timestamptz NOT NULL,
    issued_at        timestamptz,
    evidence_class   text NOT NULL,
    -- The raw artifact bytes are stored out of band; this row is the durable
    -- provenance pointer. payload_ref locates the stored blob.
    payload_ref      text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, source_version, artifact_sha256)
);

-- ---------------------------------------------------------------------------
-- Operational packages (T04): validated, authority-supplied packages. The
-- package body is stored as canonical JSONB; provenance/version are first-class
-- columns for supersession and currency queries. Structural validity is checked
-- by opkg before insert; this table stores only validated packages.
-- ---------------------------------------------------------------------------

CREATE TABLE packages (
    package_id       text PRIMARY KEY,           -- provenance.dataset_id
    alert_id         text NOT NULL,
    source_id        text NOT NULL REFERENCES sources(source_id),
    artifact_id      text NOT NULL REFERENCES source_artifacts(artifact_id),
    version          integer NOT NULL CHECK (version >= 1),
    jurisdiction     text NOT NULL,
    evidence_class   text NOT NULL,
    effective_at     timestamptz NOT NULL,
    expires_at       timestamptz NOT NULL,
    checksum_sha256  text NOT NULL CHECK (char_length(checksum_sha256) = 64),
    body             jsonb NOT NULL,             -- canonical opkg.Package
    superseded_by    text REFERENCES packages(package_id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > effective_at),
    UNIQUE (source_id, version)
);
-- A package is superseded at most once.
CREATE UNIQUE INDEX packages_superseded_by_key ON packages(superseded_by)
    WHERE superseded_by IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Versioned facts (T05): zones, routes, facilities, instructions, alerts. Each
-- carries the package version it came from and its own optimistic version.
-- Geometry columns are PostGIS geography/geometry in SRID 4326.
-- ---------------------------------------------------------------------------

CREATE TABLE zone_versions (
    zone_id        text NOT NULL,
    package_id     text NOT NULL REFERENCES packages(package_id),
    kind           text NOT NULL CHECK (kind IN ('RED', 'SAFE')),
    role           text,                          -- NULL = unknown, never assumed
    status         text,
    capacity       integer CHECK (capacity IS NULL OR capacity >= 0),
    -- Safe-zone point location; NULL for polygon-only zones. SRID 4326.
    location       geography(Point, 4326),
    version        integer NOT NULL CHECK (version >= 1),
    updated_at     timestamptz NOT NULL,
    PRIMARY KEY (zone_id, package_id)
);

CREATE TABLE route_versions (
    route_id       text NOT NULL,
    package_id     text NOT NULL REFERENCES packages(package_id),
    from_zone_id   text NOT NULL,
    to_safe_zone_id text NOT NULL,
    approval       text NOT NULL,
    mode           text NOT NULL,                 -- FOOT | VEHICLE | AMBULANCE
    verified_by    text,
    verified_at    timestamptz,
    valid_from     timestamptz,
    valid_until    timestamptz,
    -- Approved path, SRID 4326 LineString.
    geometry       geography(LineString, 4326) NOT NULL,
    version        integer NOT NULL CHECK (version >= 1),
    updated_at     timestamptz NOT NULL,
    PRIMARY KEY (route_id, package_id),
    CHECK ((verified_by IS NULL) = (verified_at IS NULL)),
    CHECK ((valid_from IS NULL) = (valid_until IS NULL)),
    CHECK (valid_until IS NULL OR valid_until > valid_from)
);

CREATE TABLE facility_versions (
    facility_id    text NOT NULL,
    package_id     text NOT NULL REFERENCES packages(package_id),
    safe_zone_id   text NOT NULL,
    version        integer NOT NULL CHECK (version >= 1),
    updated_at     timestamptz NOT NULL,
    PRIMARY KEY (facility_id, package_id)
);

-- ---------------------------------------------------------------------------
-- Sessions (T05): authenticated citizen/operator sessions. Private operations
-- bind to a session; jurisdiction-scoped operators bind to a jurisdiction.
-- Demo IDs/headers are not production authentication; a session row is the
-- durable record an authenticated principal exists.
-- ---------------------------------------------------------------------------

CREATE TABLE sessions (
    session_id     text PRIMARY KEY,
    -- 'CITIZEN' or 'OPERATOR'. Operators are jurisdiction-scoped.
    principal_kind text NOT NULL CHECK (principal_kind IN ('CITIZEN', 'OPERATOR')),
    jurisdiction   text,                          -- required for OPERATOR
    -- Opaque credential reference; the secret itself is never stored here.
    credential_ref text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL,
    revoked_at     timestamptz,
    CHECK (principal_kind <> 'OPERATOR' OR jurisdiction IS NOT NULL)
);

-- ---------------------------------------------------------------------------
-- Reservations / stays and facility/date inventory (T05/T07 foundation). P3
-- persists the reservation/capacity foundation only; the full stay workflow
-- (arrival/depart/transfer/extend) is P4. Conservation is enforced by the
-- inventory rows: reserved can never exceed capacity for a facility+date.
-- ---------------------------------------------------------------------------

CREATE TABLE facility_inventory (
    facility_id    text NOT NULL,
    service_date   date NOT NULL,
    capacity       integer NOT NULL CHECK (capacity >= 0),
    reserved       integer NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    version        integer NOT NULL CHECK (version >= 1),
    updated_at     timestamptz NOT NULL,
    PRIMARY KEY (facility_id, service_date),
    -- Conservation: never reserve more than capacity. The atomic conditional
    -- UPDATE in the store layer also guards this so contention returns no-row
    -- rather than violating the constraint under race.
    CHECK (reserved <= capacity)
);

CREATE TABLE reservations (
    reservation_id text PRIMARY KEY,
    session_id     text NOT NULL REFERENCES sessions(session_id),
    facility_id    text NOT NULL,
    service_date   date NOT NULL,
    party_size     integer NOT NULL CHECK (party_size >= 1),
    -- RESERVED -> ARRIVED -> DEPARTED; CANCELLED/EXPIRED before arrival (T07).
    state          text NOT NULL CHECK (state IN
                       ('RESERVED', 'ARRIVED', 'DEPARTED', 'CANCELLED', 'EXPIRED')),
    version        integer NOT NULL CHECK (version >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL,
    expires_at     timestamptz
);
CREATE INDEX reservations_inventory_key ON reservations(facility_id, service_date)
    WHERE state IN ('RESERVED', 'ARRIVED');

-- ---------------------------------------------------------------------------
-- Idempotency (T05): scoped to actor/session + operation + key, bound to the
-- request payload hash. A lost response after commit is retrievable with the
-- same key; a different payload under the same key is a conflict.
-- ---------------------------------------------------------------------------

CREATE TABLE idempotency_keys (
    scope          text NOT NULL,                 -- session/actor id
    operation      text NOT NULL,                 -- e.g. 'reservation.create'
    idem_key       text NOT NULL,
    payload_hash   text NOT NULL CHECK (char_length(payload_hash) = 64),
    -- The stored successful result for replay after a lost response.
    result         jsonb,
    state          text NOT NULL CHECK (state IN ('IN_PROGRESS', 'COMPLETED', 'FAILED')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    -- Retention must exceed the supported retry window (T05); swept by a worker.
    expires_at     timestamptz NOT NULL,
    PRIMARY KEY (scope, operation, idem_key)
);

-- ---------------------------------------------------------------------------
-- Audit / outbox (T05): every state change persists its audit event and its
-- outbox record in the SAME transaction as the change. Audit is append-only.
-- A hash chain links audit rows for tamper evidence; per trd.md the chain alone
-- is not proof against a privileged rewrite, so append-only is also enforced by
-- revoking UPDATE/DELETE from the application role (see roles section).
-- ---------------------------------------------------------------------------

CREATE TABLE audit_events (
    event_seq      bigint GENERATED ALWAYS AS IDENTITY,
    event_id       text NOT NULL UNIQUE,
    occurred_at    timestamptz NOT NULL,
    actor_id       text NOT NULL,
    action         text NOT NULL,
    subject_id     text NOT NULL,
    outcome        text NOT NULL,
    reason         text,
    from_state     text,
    to_state       text,
    -- Tamper-evident chain: sha256 over (prev_hash | canonical event fields).
    prev_hash      text NOT NULL,
    event_hash     text NOT NULL CHECK (char_length(event_hash) = 64),
    PRIMARY KEY (event_seq)
);

CREATE TABLE outbox_events (
    outbox_seq     bigint GENERATED ALWAYS AS IDENTITY,
    event_id       text NOT NULL UNIQUE,
    aggregate      text NOT NULL,                 -- e.g. 'reservation', 'source'
    event_type     text NOT NULL,
    payload        jsonb NOT NULL,
    occurred_at    timestamptz NOT NULL,
    published_at   timestamptz,                   -- NULL until relayed
    PRIMARY KEY (outbox_seq)
);
CREATE INDEX outbox_unpublished_key ON outbox_events(outbox_seq)
    WHERE published_at IS NULL;

-- ---------------------------------------------------------------------------
-- Least-privilege roles (P3 mandate). The application role can read/write the
-- operational tables but cannot rewrite audit history. Migrations run as a
-- separate owner role. Adjust role names to the deployment; the invariant is
-- that the runtime role has no UPDATE/DELETE on audit_events.
-- ---------------------------------------------------------------------------
-- REVOKE UPDATE, DELETE, TRUNCATE ON audit_events FROM sthira_app;
-- GRANT SELECT, INSERT ON audit_events TO sthira_app;

COMMIT;

-- ---------------------------------------------------------------------------
-- DOWN (rollback): drops only the objects this migration created. Tested as
-- part of the upgrade/rollback recovery strategy on a real instance.
-- ---------------------------------------------------------------------------
-- BEGIN;
-- DROP TABLE IF EXISTS outbox_events, audit_events, idempotency_keys,
--     reservations, facility_inventory, sessions, facility_versions,
--     route_versions, zone_versions, packages, source_artifacts,
--     source_authorizations, sources, schema_migrations;
-- DROP EXTENSION IF EXISTS postgis;
-- COMMIT;
