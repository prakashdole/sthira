package httpserver

import (
	"bytes"
	"context"
	"errors"

	"sthira/backend/internal/offlinedelivery"
	"sthira/backend/internal/store"
)

type storePublicationAdapter struct {
	st *store.Store
}

// NewStorePublicationSource constructs an offlinedelivery.PublicationSource backed by store.Store.
func NewStorePublicationSource(st *store.Store) offlinedelivery.PublicationSource {
	return &storePublicationAdapter{st: st}
}

func (a *storePublicationAdapter) GetManifest(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error) {
	m, err := a.st.GetPublishedManifest(ctx, jurisdiction)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, offlinedelivery.ErrNotFound
		}
		return nil, err
	}
	if m.Quarantined {
		return nil, offlinedelivery.ErrQuarantined
	}
	if m.SourceStatus != "CURRENT" {
		return nil, offlinedelivery.ErrNotFound
	}
	return &offlinedelivery.ManifestRecord{
		Jurisdiction:   m.Jurisdiction,
		Revision:       m.Revision,
		RawJSON:        m.RawJSON,
		ChecksumSHA256: m.ChecksumSHA256,
		SourceStatus:   m.SourceStatus,
	}, nil
}

func (a *storePublicationAdapter) GetCard(ctx context.Context, packageID string, version int) (*offlinedelivery.CardRecord, error) {
	c, err := a.st.GetPublishedCard(ctx, packageID, version)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, offlinedelivery.ErrNotFound
		}
		return nil, err
	}
	if c.Quarantined {
		return nil, offlinedelivery.ErrQuarantined
	}
	if c.SourceStatus != "CURRENT" {
		return nil, offlinedelivery.ErrNotFound
	}
	return &offlinedelivery.CardRecord{
		PackageID:      c.PackageID,
		Version:        c.Version,
		RawJSON:        c.RawJSON,
		ChecksumSHA256: c.ChecksumSHA256,
		SourceStatus:   c.SourceStatus,
	}, nil
}

func (a *storePublicationAdapter) GetResource(ctx context.Context, resourceID string) (*offlinedelivery.ResourceContent, error) {
	r, err := a.st.GetPublishedResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, offlinedelivery.ErrNotFound
		}
		return nil, err
	}
	return &offlinedelivery.ResourceContent{
		Reader:         nopCloserSeeker{bytes.NewReader(r.Content)},
		ContentType:    r.ContentType,
		ContentLength:  r.ContentLength,
		ChecksumSHA256: r.ChecksumSHA256,
		ETag:           `"sha256-` + r.ChecksumSHA256 + `"`,
	}, nil
}

type nopCloserSeeker struct {
	*bytes.Reader
}

func (nopCloserSeeker) Close() error { return nil }
