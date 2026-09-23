// Command sthira-exercise runs the bounded /api/v3 backend in an
// ISOLATED, PROCESS-CONTROLLED exercise mode for the round-two demo.
//
// Differences from cmd/sthira:
//
//  1. The synthetic exercise dependency is wired ON at process start
//     (WithSyntheticExercise(StaticSyntheticExercise(true))). Production
//     cmd/sthira defaults to OFF. No request header, body field or query
//     parameter can activate synthetic behaviour; it is decided by the
//     binary you start.
//
//  2. On first boot, when STHIRA_EXERCISE_SEED=1 is set, the binary
//     inserts (or replaces) a labelled SYNTHETIC_DEMO source,
//     authorization, package, safe zone, facility, route into the
//     configured database, using a fixed exercise fixture (one red
//     zone, one OPEN safe zone, one facility, one verified
//     SYNTHETIC_DEMO route, allocation_policy.order = [safe-zone id]).
//     Idempotent: re-running drops and re-inserts the exercise IDs.
//
//  3. The exercise label is logged loudly at startup. Operational
//     guidance is never claimable from this binary.
//
// This is NOT production. Production cmd/sthira does not import this
// file; this binary exists so the demo can speak against real Go
// handlers, a real durable store and the synthetic data the demo
// requires, while keeping the same trust boundaries (D47) that the
// production binary enforces.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sthira/backend/internal/httpserver"
	"sthira/backend/internal/store"
)

// exerciseBody is the typed Package body the seed inserts into the
// database. ONE safe zone, ONE facility in it, ONE verified
// SYNTHETIC_DEMO route bound to it, an English + ml-IN instruction
// set, and an allocation_policy.order that lists the safe-zone ID
// (per the opkg validator and the R01 fix).
const exerciseBody = `{
	"red_zones":[
		{"id":"RZDEMO-1"}
	],
	"safe_zones":[
		{"id":"SZDEMO-1","status":"OPEN"}
	],
	"approved_routes":[
		{
			"id":"RTDEMO-1",
			"from_zone_id":"RZDEMO-1",
			"to_safe_zone_id":"SZDEMO-1",
			"approval":"SYNTHETIC_DEMO",
			"mode":"FOOT",
			"verified_by":"exercise.demo",
			"valid_from":"2026-01-01T00:00:00Z",
			"valid_until":"2030-01-01T00:00:00Z",
			"geometry":{"type":"LineString","coordinates":[[76.10,11.55],[76.12,11.55]]}
		}
	],
	"facilities":[
		{"id":"FACDEMO-1","safe_zone_id":"SZDEMO-1"}
	],
	"instruction_assets":[
		{"id":"INSDEMO-EN","language":"en-IN","title":"Demo instructions","summary":"Move to the demo safe zone."},
		{"id":"INSDEMO-ML","language":"ml-IN","title":"ഡെമോ നിർദ്ദേശം","summary":"ഡെമോ സുരക്ഷിത മേഖലയിലേക്ക് പോകുക."}
	],
	"allocation_policy":{
		"order":["SZDEMO-1"],
		"reservation_expiry_seconds":3600,
		"temporary_stay_min_days":1,
		"temporary_stay_max_days":14,
		"allow_walk_ins":true,
		"allow_transfers":true,
		"route_required":false
	},
	"emergency_contacts":[
		{"name":"Emergency","number":"112"}
	]
}`

const (
	exerciseJurisdiction = "DEMO-EXERCISE"
	exerciseSourceID     = "SRCDEMO-1"
	exerciseArtifactID   = "ARTDEMO-1"
	exercisePackageID    = "PKGDEMO-1"
	exerciseAuthID       = "AUTHDEMO-1"
	exerciseRouteID      = "RTDEMO-1"
	exerciseZoneID       = "SZDEMO-1"
	exerciseRedZoneID    = "RZDEMO-1"
	exerciseFacilityID   = "FACDEMO-1"
)

// seedExercise drops the existing exercise rows (if any) and inserts
// the demo package. Idempotent: safe to call on every start when
// STHIRA_EXERCISE_SEED=1.
func seedExercise(ctx context.Context, st *store.Store) error {
	body := []byte(exerciseBody)
	var pb struct {
		AllocationPolicy struct {
			Order []string `json:"order"`
		} `json:"allocation_policy"`
	}
	if err := json.Unmarshal(body, &pb); err != nil {
		return fmt.Errorf("parse exercise body: %w", err)
	}
	if len(pb.AllocationPolicy.Order) == 0 {
		return errors.New("exercise body missing allocation_policy.order (opkg requires it)")
	}

	now := time.Now().UTC()
	sources := store.NewSourceStore(store.ChainAuditor{})
	hash := sha256.Sum256(body)

	return st.InTx(ctx, func(tx store.DBTX) error {
		// Clean prior rows so re-seed is idempotent.
		for _, q := range []string{
			`DELETE FROM facilities WHERE facility_id = $1`,
			`DELETE FROM zone_versions WHERE package_id = $1`,
			`DELETE FROM packages WHERE package_id = $1`,
			`DELETE FROM source_artifacts WHERE artifact_id = $1`,
			`DELETE FROM source_authorizations WHERE authorization_id = $1`,
			`DELETE FROM sources WHERE source_id = $1`,
		} {
			switch q {
			case `DELETE FROM facilities WHERE facility_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exerciseFacilityID); err != nil {
					return err
				}
			case `DELETE FROM zone_versions WHERE package_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exercisePackageID); err != nil {
					return err
				}
			case `DELETE FROM packages WHERE package_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exercisePackageID); err != nil {
					return err
				}
			case `DELETE FROM source_artifacts WHERE artifact_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exerciseArtifactID); err != nil {
					return err
				}
			case `DELETE FROM source_authorizations WHERE authorization_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exerciseAuthID); err != nil {
					return err
				}
			case `DELETE FROM sources WHERE source_id = $1`:
				if _, err := tx.ExecContext(ctx, q, exerciseSourceID); err != nil {
					return err
				}
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			 VALUES ($1,'exercise.demo','demo.example','OPERATIONAL',1,$2,$2)`,
			exerciseSourceID, now); err != nil {
			return err
		}
		if err := sources.RecordAuthorization(ctx, tx, store.Authorization{
			AuthorizationID: exerciseAuthID,
			SourceID:        exerciseSourceID,
			GrantedBy:       "exercise.demo",
			EvidenceRef:     "EXERCISE-NO-REAL-AUTHORITY",
			Jurisdiction:    exerciseJurisdiction,
			GrantedAt:       now,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			 VALUES ($1,$2,1,$3,$4,'SYNTHETIC_DEMO',$5)`,
			exerciseArtifactID, exerciseSourceID, hex.EncodeToString(hash[:]), now, "memory://exercise"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			 VALUES ($1,'ALT-EX',$2,$3,1,$4,'SYNTHETIC_DEMO',$5,$6,$7,$8)`,
			exercisePackageID, exerciseSourceID, exerciseArtifactID, exerciseJurisdiction,
			now.Add(-time.Hour), now.Add(24*time.Hour), hex.EncodeToString(hash[:]), body); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			 VALUES ($1,$2,'SAFE','SAFE','OPEN',100,1,$3)`,
			exerciseZoneID, exercisePackageID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			 VALUES ($1,$2,$3,'Asia/Kolkata',1,$4)`,
			exerciseFacilityID, exercisePackageID, exerciseZoneID, now); err != nil {
			return err
		}
		// The package body references RTDEMO-1; mirror it in the
		// route_versions table so store-level reservation revalidation
		// can find the route bound to the facility's safe zone.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO route_versions
				(route_id, package_id, from_zone_id, to_safe_zone_id, approval, mode,
				 verified_by, verified_at, valid_from, valid_until, geometry, version, updated_at)
			 VALUES ($1,$2,$3,$4,'SYNTHETIC_DEMO','FOOT','exercise.demo',$5,$6,$7,
			         ST_GeogFromText('SRID=4326;LINESTRING(76.10 11.55, 76.12 11.56)'),1,$5)`,
			exerciseRouteID, exercisePackageID, exerciseRedZoneID, exerciseZoneID, now,
			"2026-01-01T00:00:00Z", "2030-01-01T00:00:00Z"); err != nil {
			return err
		}

		// Seed inventory for a 30-day window starting today so the demo
		// can commit a reservation without an extra setup step.
		for d := 0; d < 30; d++ {
			day := now.AddDate(0, 0, d).Format("2006-01-02")
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO facility_inventory (facility_id, service_date, capacity, held, occupied, version, updated_at)
				 VALUES ($1,$2::date,100,0,0,1,$3)`,
				exerciseFacilityID, day, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Warn("STHIRA-EXERCISE-MODE: synthetic data only, no real authority")

	addr := os.Getenv("STHIRA_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	dsn := os.Getenv("STHIRA_DATABASE_DSN")
	if dsn == "" {
		logger.Error("STHIRA_DATABASE_DSN is required for the exercise binary")
		os.Exit(2)
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.Scheme != "postgres" && u.Scheme != "postgresql" {
		logger.Error("unsupported DSN scheme", "scheme", u.Scheme)
		os.Exit(2)
	}

	st, err := store.Open(dsn)
	if err != nil {
		logger.Error("open store", "error", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if os.Getenv("STHIRA_EXERCISE_SEED") == "1" {
		if err := seedExercise(ctx, st); err != nil {
			logger.Error("seed exercise", "error", err)
			os.Exit(1)
		}
		logger.Info("exercise fixture seeded", "jurisdiction", exerciseJurisdiction, "package", exercisePackageID)
	}

	cfg := httpserver.DefaultConfig(addr)
	srv := httpserver.New(cfg,
		httpserver.WithLogger(logger),
		httpserver.WithStore(st),
		httpserver.WithProber(store.NewReadinessProber(st.DB(), store.SchemaRevision)),
		httpserver.WithPersistedContextResolver(st),
		httpserver.WithSyntheticExercise(httpserver.StaticSyntheticExercise(true)),
	)
	logger.Info("sthira-exercise listening (synthetic ON)", "addr", addr, "schema_revision", store.SchemaRevision)
	if err := srv.Serve(ctx); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
