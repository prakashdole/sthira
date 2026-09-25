package capfeed

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fixtureFetcher is a scripted transport for deterministic tests.
type fixtureFetcher struct {
	responses []Response
	errs      []error
	calls     int
	// seenETags records the etag argument passed on each call.
	seenETags []string
}

func (f *fixtureFetcher) fetch(ctx context.Context, uri, etag string) (Response, error) {
	f.seenETags = append(f.seenETags, etag)
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return Response{}, f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return Response{}, errors.New("no scripted response")
}

func TestTransportUpdated(t *testing.T) {
	ff := &fixtureFetcher{responses: []Response{
		{Status: 200, Body: []byte("<alert/>"), ETag: "v1"},
	}}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	res, err := tr.Refresh(context.Background(), "fixture:cap")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != FetchUpdated {
		t.Errorf("state = %q", res.State)
	}
	if string(res.Body) != "<alert/>" {
		t.Errorf("body = %q", res.Body)
	}
	if res.ETag != "v1" {
		t.Errorf("etag = %q", res.ETag)
	}
	if res.RetrievedAt.IsZero() {
		t.Error("RetrievedAt not set from injected clock")
	}
}

func TestTransportConditionalSendsETag(t *testing.T) {
	ff := &fixtureFetcher{responses: []Response{
		{Status: 200, Body: []byte("x"), ETag: "v1"},
		{Status: 304},
	}}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	if _, err := tr.Refresh(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}
	res, err := tr.Refresh(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != FetchNotModified {
		t.Errorf("state = %q", res.State)
	}
	// Second call must have sent the stored validator.
	if len(ff.seenETags) != 2 || ff.seenETags[1] != "v1" {
		t.Errorf("seenETags = %v", ff.seenETags)
	}
}

func TestTransportStaleCachePreserved(t *testing.T) {
	ff := &fixtureFetcher{
		responses: []Response{{Status: 200, Body: []byte("cached"), ETag: "v1"}},
		errs:      []error{nil, errors.New("boom"), errors.New("boom"), errors.New("boom")},
	}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	tr.BaseDelay = time.Millisecond
	if _, err := tr.Refresh(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}
	// All subsequent attempts fail -> stale cache preserved, no error.
	res, err := tr.Refresh(context.Background(), "u")
	if err != nil {
		t.Fatalf("stale cache must not error, got %v", err)
	}
	if res.State != FetchStale {
		t.Errorf("state = %q, want STALE_CACHE", res.State)
	}
}

func TestTransportUnavailableNoCache(t *testing.T) {
	ff := &fixtureFetcher{errs: []error{errors.New("boom"), errors.New("boom"), errors.New("boom")}}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	tr.BaseDelay = time.Millisecond
	res, err := tr.Refresh(context.Background(), "u")
	if err == nil {
		t.Error("want error when unavailable with no cache")
	}
	if res.State != FetchUnavailable {
		t.Errorf("state = %q", res.State)
	}
	if res.Attempts != 3 {
		t.Errorf("attempts = %d, want 3 (bounded retries)", res.Attempts)
	}
}

func TestTransportRetriesThenSucceeds(t *testing.T) {
	ff := &fixtureFetcher{
		responses: []Response{{}, {Status: 200, Body: []byte("ok"), ETag: "v2"}},
		errs:      []error{errors.New("transient"), nil},
	}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	tr.BaseDelay = time.Millisecond
	res, err := tr.Refresh(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != FetchUpdated || res.Attempts != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestTransport304WithoutCacheIsFailure(t *testing.T) {
	ff := &fixtureFetcher{responses: []Response{{Status: 304}, {Status: 304}, {Status: 304}}}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	tr.BaseDelay = time.Millisecond
	res, err := tr.Refresh(context.Background(), "u")
	if err == nil {
		t.Error("304 with no cache must be an error")
	}
	if res.State != FetchUnavailable {
		t.Errorf("state = %q", res.State)
	}
}

func TestTransportEmpty200Retried(t *testing.T) {
	ff := &fixtureFetcher{responses: []Response{
		{Status: 200, Body: nil},
		{Status: 200, Body: []byte("real"), ETag: "v3"},
	}}
	tr := NewTransport(ff.fetch, func() time.Time { return fixedNow })
	tr.BaseDelay = time.Millisecond
	res, err := tr.Refresh(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != FetchUpdated || string(res.Body) != "real" {
		t.Errorf("res = %+v", res)
	}
}

func TestTransportNilDepsPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want panic on nil fetcher")
		}
	}()
	NewTransport(nil, func() time.Time { return fixedNow })
}
