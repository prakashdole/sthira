-- P4 stays: destination choice and immediate/temporary stays. Additive only.
--
-- Extends the P3 reservation/capacity foundation to the full stay lifecycle
-- (reserve -> arrive -> depart, plus cancel/expire/extend/transfer/correct)
-- with date-range capacity conservation per plan/equations.md:
--   E = free + held + occupied;  reserve: free-=p,held+=p;
--   arrive: held-=p,occupied+=p (no second decrement);
--   cancel/expire: held-=p,free+=p;  depart: occupied-=p,free+=p.
--
-- Stay dates are HALF-OPEN [start_date, end_date) in FACILITY local-date
-- semantics with an explicit facility timezone (trd.md line 27). A stay from
-- 2026-09-20 to 2026-09-22 occupies the 20th and 21st, not the 22nd.
--
-- Target: PostgreSQL 15+ with PostGIS 3.x. Apply inside a transaction.

BEGIN;

-- Migration tracking.
INSERT INTO schema_migrations (revision) VALUES (2)
ON CONFLICT (revision) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Facilities: durable facility records with an explicit timezone for stay-date
-- semantics. Facilities originate from operational packages (synthetic while
-- O01/O07 are blocked). capacity_mode and accessibility stay unknown when the
-- source does not supply them; they are never inferred.
-- ---------------------------------------------------------------------------

CREATE TABLE facilities (
    facility_id    text PRIMARY KEY,
    package_id     text NOT NULL REFERENCES packages(package_id),
    safe_zone_id   text NOT NULL,
    -- IANA timezone name (e.g. 'Asia/Kolkata') governing local stay dates.
    timezone       text NOT NULL,
    -- Capacity mode and accessibility are optional; NULL = unknown, never
    -- assumed. Displayed honestly as unknown (R03, R11).
    capacity_mode  text,
    accessibility  text,
    version        integer NOT NULL CHECK (version >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL
);

-- ---------------------------------------------------------------------------
-- Inventory: replace the P3 reserved-only model with free/held/occupied per
-- facility+date. P3 created facility_inventory(capacity, reserved); P4 needs
-- the three-bucket model. We add held/occupied and keep reserved as a
-- transitional alias equal to held (P3 wrote only RESERVED holds). The
-- conservation invariant is free+held+occupied = effective capacity.
-- ---------------------------------------------------------------------------

ALTER TABLE facility_inventory
    ADD COLUMN held     integer NOT NULL DEFAULT 0 CHECK (held >= 0),
    ADD COLUMN occupied integer NOT NULL DEFAULT 0 CHECK (occupied >= 0);

-- Migrate P3 'reserved' holds into 'held' (P3 only ever created holds).
UPDATE facility_inventory SET held = reserved WHERE reserved <> held;

-- free is derived: capacity - held - occupied. Conservation: never negative.
ALTER TABLE facility_inventory
    ADD CONSTRAINT inventory_conservation CHECK (held + occupied <= capacity);

-- ---------------------------------------------------------------------------
-- Stays: the durable stay record. A reservation creates a stay in RESERVED;
-- arrival, departure, cancellation, expiry, extension and transfer are audited
-- transitions. Half-open [start_date, end_date) in the facility's timezone.
-- ---------------------------------------------------------------------------

CREATE TABLE stays (
    stay_id        text PRIMARY KEY,
    reservation_id text NOT NULL REFERENCES reservations(reservation_id),
    session_id     text NOT NULL REFERENCES sessions(session_id),
    facility_id    text NOT NULL REFERENCES facilities(facility_id),
    party_size     integer NOT NULL CHECK (party_size >= 1),
    -- Half-open local-date interval [start_date, end_date) in the facility tz.
    start_date     date NOT NULL,
    end_date       date NOT NULL,
    state          text NOT NULL CHECK (state IN
                       ('RESERVED','ARRIVED','DEPARTED','CANCELLED','EXPIRED')),
    -- The source/package snapshot this stay was validated against, for
    -- revalidation at commit and stale-selection detection.
    package_id     text NOT NULL REFERENCES packages(package_id),
    route_id       text,                          -- NULL when no route chosen
    -- Transfer linkage: a stay created by a transfer references its origin.
    transferred_from text REFERENCES stays(stay_id),
    version        integer NOT NULL CHECK (version >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL,
    expires_at     timestamptz,                   -- hold expiry (policy)
    arrived_at     timestamptz,
    departed_at    timestamptz,
    CHECK (end_date > start_date)
);
CREATE INDEX stays_inventory_key ON stays(facility_id, start_date, end_date)
    WHERE state IN ('RESERVED','ARRIVED');
CREATE INDEX stays_session_key ON stays(session_id);

-- ---------------------------------------------------------------------------
-- Place aliases: jurisdiction-scoped name/alias -> place resolution. Multiple
-- aliases may map to one place; one name may be ambiguous across places.
-- Resolution returns candidates or an explicit ambiguity, never a guess.
-- ---------------------------------------------------------------------------

CREATE TABLE place_aliases (
    alias_id       text PRIMARY KEY,
    jurisdiction   text NOT NULL,
    -- Normalized lookup key (lowercased, trimmed) for the alias or admin ID.
    lookup_key     text NOT NULL,
    -- The place this alias resolves to (a zone/facility id in the package).
    place_id       text NOT NULL,
    place_kind     text NOT NULL CHECK (place_kind IN ('ZONE','FACILITY','ADMIN')),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX place_aliases_lookup ON place_aliases(jurisdiction, lookup_key);

-- ---------------------------------------------------------------------------
-- Route landmarks / non-map guidance: ordered, mode-specific steps paired with
-- each route (R10). Equivalent non-map instructions are first-class so guidance
-- never depends on rendering geometry.
-- ---------------------------------------------------------------------------

CREATE TABLE route_landmarks (
    route_id       text NOT NULL,
    seq            integer NOT NULL CHECK (seq >= 1),
    -- Ordered landmark label and the equivalent non-map instruction text.
    landmark       text NOT NULL,
    instruction    text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (route_id, seq)
);

-- ---------------------------------------------------------------------------
-- Route closures: a closure invalidates affected guidance (R10). A closed route
-- cannot satisfy the operational route gate.
-- ---------------------------------------------------------------------------

CREATE TABLE route_closures (
    closure_id     text PRIMARY KEY,
    route_id       text NOT NULL,
    closed_at      timestamptz NOT NULL,
    reason         text,
    -- NULL reopened_at = currently closed.
    reopened_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX route_closures_route ON route_closures(route_id) WHERE reopened_at IS NULL;

COMMIT;

-- ---------------------------------------------------------------------------
-- DOWN (rollback): drops only the objects this migration created.
-- ---------------------------------------------------------------------------
-- BEGIN;
-- DROP TABLE IF EXISTS route_closures, route_landmarks, place_aliases, stays,
--     facilities;
-- ALTER TABLE facility_inventory DROP CONSTRAINT IF EXISTS inventory_conservation;
-- ALTER TABLE facility_inventory DROP COLUMN IF EXISTS held, DROP COLUMN IF EXISTS occupied;
-- DELETE FROM schema_migrations WHERE revision = 2;
-- COMMIT;
