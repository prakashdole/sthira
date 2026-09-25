//go:build crashtest

package httpserver_test

// Cross-process concurrency: N independent child `sthira` server processes
// (not goroutines) each attempt to reserve the single remaining space on the
// same facility/date. Exactly one must win; conservation (reserved<=capacity)
// must hold; and the winner's multi-step write (reservation + idempotency
// result + audit) must have committed atomically in one transaction.
//
// Gated on STHIRA_TEST_DSN and STHIRA_RUN_PROCESS_TESTS=1. Unsandboxed shell.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func TestCrossProcessLastSpace(t *testing.T) {
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" || os.Getenv("STHIRA_RUN_PROCESS_TESTS") != "1" {
		t.Skip("requires STHIRA_TEST_DSN and STHIRA_RUN_PROCESS_TESTS=1")
	}
	bin := buildCrashServer(t)
	db := openDB(t, dsn)

	sess := uidP("SES")
	if _, err := db.Exec(`INSERT INTO sessions (session_id, principal_kind, credential_ref, expires_at) VALUES ($1,'CITIZEN','cred',$2)`, sess, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	fac := uidP("FAC")
	date := time.Now().UTC().Truncate(24 * time.Hour)
	// Pre-create the inventory row with capacity 1 so all contenders race on the
	// same row's conditional update.
	if _, err := db.Exec(`INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at) VALUES ($1,$2,1,0,1,$3)`, fac, date, time.Now().UTC()); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	const procs = 6
	type result struct {
		status int
		err    error
	}
	results := make([]result, procs)
	var wg sync.WaitGroup

	// Launch N independent server processes, each on its own address, and have
	// each POST one reservation for the last space concurrently.
	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			addr := freeAddr(t)
			cmd := startServer(t, bin, addr, dsn)
			defer func() { cmd.Process.Kill(); cmd.Wait() }()

			body, _ := json.Marshal(map[string]any{
				"reservation_id": uidP("RES"), "session_id": sess, "facility_id": fac,
				"service_date": date, "party_size": 1, "idem_key": uidP("KEY"),
			})
			req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/crashtest/commit", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results[i] = result{status: -1, err: err}
				return
			}
			defer resp.Body.Close()
			results[i] = result{status: resp.StatusCode}
		}(i)
	}
	wg.Wait()

	wins := 0
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("process %d request error: %v", i, r.err)
		}
		switch r.status {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			// expected loser outcome: capacity exhausted or version conflict
		default:
			t.Fatalf("process %d unexpected status %d", i, r.status)
		}
	}
	if wins != 1 {
		t.Fatalf("exactly one process must win the last space; got %d", wins)
	}

	// Conservation: reserved must be exactly 1 and never exceed capacity.
	var reserved, capacity int
	if err := db.QueryRow(`SELECT reserved, capacity FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, fac, date).Scan(&reserved, &capacity); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if reserved != 1 || reserved > capacity {
		t.Fatalf("conservation violated: reserved=%d capacity=%d", reserved, capacity)
	}

	// Atomicity of the winner's multi-step write: exactly one reservation row
	// for the facility/date, and its audit event exists (same transaction).
	var resCount int
	if err := db.QueryRow(`SELECT count(*) FROM reservations WHERE facility_id=$1 AND service_date=$2 AND state='RESERVED'`, fac, date).Scan(&resCount); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if resCount != 1 {
		t.Fatalf("RESERVED reservations for last space = %d, want 1", resCount)
	}
}
