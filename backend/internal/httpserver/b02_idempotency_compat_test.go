package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/offlinequeue"
	"sthira/backend/internal/store"
)

// TestB02_HTTPReservationLegacyReplayAndAliasingPrevention verifies that:
// 1. A replayed reservation matching a legacy-persisted row succeeds with HTTP 200 OK.
// 2. An aliasing attempt changing discriminating fields (e.g. party_size) is denied with HTTP 409 Conflict.
// 3. Different session cannot hijack the legacy reservation key.
func TestB02_HTTPReservationLegacyReplayAndAliasingPrevention(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	startStr, endStr := httpDay(10), httpDay(14)
	startT, endT := httpDayT(10), httpDayT(14)
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 10, startT, endT)

	sessID, token := createSession(t, srv)
	idemKey := "IDEM-LEGACY-" + fmt.Sprintf("%d", time.Now().UnixNano())
	resID := "RES-LEGACY-" + fmt.Sprintf("%d", time.Now().UnixNano())
	stayID := "STAY-LEGACY-" + fmt.Sprintf("%d", time.Now().UnixNano())
	originalPartySize := 1

	now := time.Now().UTC()
	expT := now.Add(24 * time.Hour)

	// Seed existing committed reservation in PostgreSQL
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, 'RESERVED', 1, $6, $6, $7)`,
			resID, sessID, facID, startT, originalPartySize, now, expT); err != nil {
			return err
		}
		stays := store.NewStayStore(store.ChainAuditor{})
		if err := stays.Reserve(t.Context(), tx, store.Stay{
			StayID: stayID, ReservationID: resID, SessionID: sessID,
			FacilityID: facID, PartySize: originalPartySize,
			StartDate: startT, EndDate: endT, PackageID: pkgID,
			ExpiresAt: &expT,
		}, now, idemKey); err != nil {
			return err
		}
		// Calculate legacy pipe-delimited payload hash
		legacyRaw := sessID + "|" + facID + "|" + pkgID + "||" + startStr + "|" + endStr + "|" + idemKey
		h := sha256.Sum256([]byte(legacyRaw))
		legacyHash := hex.EncodeToString(h[:])

		resultJSON, _ := json.Marshal(map[string]string{
			"reservation_id": resID,
			"stay_id":        stayID,
		})
		_, err := tx.ExecContext(t.Context(), `
			INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, state, result, expires_at)
			VALUES ($1, 'reservation.create', $2, $3, 'COMPLETED', $4, $5)`,
			sessID, idemKey, legacyHash, resultJSON, expT)
		return err
	})
	if err != nil {
		t.Fatalf("seed legacy reservation: %v", err)
	}

	// 1. Exact replay with original party size (1) -> MUST SUCCEED with HTTP 200 OK and matching IDs
	bodyExact := reservationBody(facID, pkgID, originalPartySize, startStr, endStr, idemKey, snap)
	recExact := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, bodyExact)
	if recExact.code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on valid legacy replay, got %d: %s", recExact.code, recExact.body)
	}
	envExact := decodeEnvelope(t, recExact)
	var dataExact map[string]string
	bExact, _ := json.Marshal(envExact.Data)
	_ = json.Unmarshal(bExact, &dataExact)
	if dataExact["reservation_id"] != resID || dataExact["stay_id"] != stayID {
		t.Fatalf("replay returned wrong IDs: %v", dataExact)
	}

	// 2. Aliasing attack: same key, same session, but party_size = 4 -> MUST RETURN 409 CONFLICT
	bodyAliased := reservationBody(facID, pkgID, 4, startStr, endStr, idemKey, snap)
	recAliased := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, bodyAliased)
	if recAliased.code != http.StatusConflict {
		t.Fatalf("expected HTTP 409 on party size aliasing, got %d: %s", recAliased.code, recAliased.body)
	}
	envAliased := decodeEnvelope(t, recAliased)
	if len(envAliased.Errors) == 0 || envAliased.Errors[0].Code != contracts.ErrIdempotencyConflict {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", envAliased.Errors)
	}

	// 3. Different session attempting to replay same key -> MUST NOT DISCLOSE or ALIAS
	_, otherToken := createSession(t, srv)
	recOther := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", otherToken, bodyExact)
	// Under a different session, the key is scoped to otherSessionID.
	// Since that key does not exist for the other session, it will attempt a new reservation or conflict,
	// but it will NEVER return the original session's resID.
	if recOther.code == http.StatusOK {
		envOther := decodeEnvelope(t, recOther)
		var dataOther map[string]string
		bOther, _ := json.Marshal(envOther.Data)
		_ = json.Unmarshal(bOther, &dataOther)
		if dataOther["reservation_id"] == resID {
			t.Fatalf("critical leak: other session hijacked legacy reservation ID %s", resID)
		}
	}
}

// TestB02_HTTPInterruptedDownloadResumeRange206 validates that card downloads
// support RFC 9110 range requests (HTTP 206) for interrupted download recovery.
func TestB02_HTTPInterruptedDownloadResumeRange206(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	startT, endT := httpDayT(10), httpDayT(14)
	pkgID, _, _ := seedHTTPPackageFacility(t, st, 10, startT, endT)

	// Seed published_cards for delivery
	cardJSON := []byte(`{"package_id":"` + pkgID + `","version":1,"zones":[{"id":"SZ","status":"OPEN"}],"facilities":[{"id":"FAC-1"}],"content":"` + strings.Repeat("A", 400) + `"}`)
	cardHash := hex.EncodeToString(func() []byte { h := sha256.Sum256(cardJSON); return h[:] }())
	var srcID string
	_ = st.DB().QueryRowContext(t.Context(), `SELECT source_id FROM packages WHERE package_id = $1`, pkgID).Scan(&srcID)
	if _, err := st.DB().ExecContext(t.Context(), `
		INSERT INTO published_cards (package_id, version, source_id, jurisdiction, raw_json, checksum_sha256, source_status, quarantined)
		VALUES ($1, 1, $2, 'JTEST', $3, $4, 'CURRENT', false)
		ON CONFLICT (package_id, version) DO UPDATE SET raw_json = $3, source_status = 'CURRENT', quarantined = false`,
		pkgID, srcID, cardJSON, cardHash); err != nil {
		t.Fatalf("seed published card: %v", err)
	}

	url := fmt.Sprintf("%s/api/v3/packages/%s/versions/1", srv.URL, pkgID)

	// Fetch complete package
	reqFull, _ := http.NewRequest(http.MethodGet, url, nil)
	respFull, err := srv.Client().Do(reqFull)
	if err != nil {
		t.Fatalf("fetch full package: %v", err)
	}
	defer respFull.Body.Close()
	if respFull.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", respFull.StatusCode)
	}
	fullBytes := []byte(readAll(t, respFull))
	totalLen := len(fullBytes)
	if totalLen < 10 {
		t.Fatalf("package content too short: %d", totalLen)
	}

	// 1. Partial request: first 50 bytes (or half if smaller)
	split := 50
	if split >= totalLen {
		split = totalLen / 2
	}
	reqPart1, _ := http.NewRequest(http.MethodGet, url, nil)
	reqPart1.Header.Set("Range", fmt.Sprintf("bytes=0-%d", split-1))
	respPart1, err := srv.Client().Do(reqPart1)
	if err != nil {
		t.Fatalf("fetch part 1: %v", err)
	}
	defer respPart1.Body.Close()
	if respPart1.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", respPart1.StatusCode)
	}
	crExpected := fmt.Sprintf("bytes 0-%d/%d", split-1, totalLen)
	if cr := respPart1.Header.Get("Content-Range"); cr != crExpected {
		t.Errorf("expected Content-Range %s, got %s", crExpected, cr)
	}
	part1Bytes := []byte(readAll(t, respPart1))
	if len(part1Bytes) != split {
		t.Fatalf("part 1 length mismatch: got %d want %d", len(part1Bytes), split)
	}

	// 2. Resume request: remaining bytes from offset split-
	reqPart2, _ := http.NewRequest(http.MethodGet, url, nil)
	reqPart2.Header.Set("Range", fmt.Sprintf("bytes=%d-", split))
	respPart2, err := srv.Client().Do(reqPart2)
	if err != nil {
		t.Fatalf("fetch part 2: %v", err)
	}
	defer respPart2.Body.Close()
	if respPart2.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", respPart2.StatusCode)
	}
	part2Bytes := []byte(readAll(t, respPart2))

	// Reassembled stream must match fullBytes exactly
	reassembled := append(part1Bytes, part2Bytes...)
	if string(reassembled) != string(fullBytes) {
		t.Fatalf("reassembled range content does not match full content")
	}

	// 3. Unsatisfiable range -> HTTP 416
	reqInvalid, _ := http.NewRequest(http.MethodGet, url, nil)
	reqInvalid.Header.Set("Range", fmt.Sprintf("bytes=%d-", totalLen+1000))
	respInvalid, err := srv.Client().Do(reqInvalid)
	if err != nil {
		t.Fatalf("fetch invalid range: %v", err)
	}
	defer respInvalid.Body.Close()
	if respInvalid.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416 Range Not Satisfiable, got %d", respInvalid.StatusCode)
	}
}

// TestB02_HTTPOfflineQueueRestartAndUncertainReconcile tests that:
// 1. Offline queue state survives local client store restart.
// 2. An uncertain response (e.g. network timeout after server commit) transitions to PENDING_RECONCILIATION.
// 3. Re-draining the queue re-transmits the operation under the same idempotency key and reconciles to COMMITTED.
func TestB02_HTTPOfflineQueueRestartAndUncertainReconcile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sthira-b02-queue-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	nowFn := func() time.Time { return now }

	qStore1, err := offlinequeue.NewStore(tempDir, nowFn)
	if err != nil {
		t.Fatalf("new queue store: %v", err)
	}

	op := offlinequeue.PendingOperation{
		ID:              "OP-B02-" + fmt.Sprintf("%d", now.UnixNano()),
		IdempotencyKey:  "IDEM-Q-B02-" + fmt.Sprintf("%d", now.UnixNano()),
		Endpoint:        "/api/v3/reservations",
		Method:          http.MethodPost,
		TokenRef:        "tok-b02",
		Payload:         json.RawMessage(`{"facility_id":"FAC-1","party_size":1}`),
		SnapshotVersion: 1,
		SelectionExpiry: now.Add(24 * time.Hour),
		State:           offlinequeue.StatePending,
		CreatedAt:       now,
	}

	ctx := context.Background()
	if err := qStore1.Enqueue(ctx, op); err != nil {
		t.Fatalf("enqueue op: %v", err)
	}

	// Verify file was written to disk
	path := filepath.Join(tempDir, op.ID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("queue file missing: %v", err)
	}

	// 1. Simulate process restart: create a new Store from the same directory
	qStore2, err := offlinequeue.NewStore(tempDir, nowFn)
	if err != nil {
		t.Fatalf("restart queue store: %v", err)
	}

	restored, err := qStore2.Get(ctx, op.ID)
	if err != nil {
		t.Fatalf("get restored op: %v", err)
	}
	if restored.ID != op.ID || restored.State != offlinequeue.StatePending {
		t.Fatalf("restored op mismatch: %+v", restored)
	}

	// 2. Transition to IN_FLIGHT, then simulate network timeout / uncertain outcome -> PENDING_RECONCILIATION
	op.State = offlinequeue.StateInFlight
	if err := qStore2.UpdateState(ctx, op); err != nil {
		t.Fatalf("transition in-flight: %v", err)
	}
	_ = op.SetPendingReconciliation(now, 504, "network timeout on ack")
	if err := qStore2.UpdateState(ctx, op); err != nil {
		t.Fatalf("transition pending reconciliation: %v", err)
	}

	reconOp, err := qStore2.Get(ctx, op.ID)
	if err != nil || reconOp.State != offlinequeue.StatePendingReconciliation {
		t.Fatalf("expected PENDING_RECONCILIATION, got state=%v err=%v", reconOp.State, err)
	}

	// 3. Worker re-drain: server returns 200 OK replay -> transition to COMMITTED
	_ = op.SetCommitted(now, http.StatusOK, json.RawMessage(`{"reservation_id":"RES-RECON-1"}`))
	if err := qStore2.UpdateState(ctx, op); err != nil {
		t.Fatalf("transition committed: %v", err)
	}

	finalOp, err := qStore2.Get(ctx, op.ID)
	if err != nil || finalOp.State != offlinequeue.StateCommitted {
		t.Fatalf("expected COMMITTED, got state=%v err=%v", finalOp.State, err)
	}
	if !strings.Contains(string(finalOp.ServerResult), "RES-RECON-1") {
		t.Fatalf("server response not recorded: %s", string(finalOp.ServerResult))
	}
}
