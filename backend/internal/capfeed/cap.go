// Package capfeed is the bounded CAP 1.2 ingestion boundary for the Go
// backend. It parses untrusted CAP XML, preserves the raw artifact, enforces
// source identity (sender allow-list), validates timestamps and geometry, and
// applies the update/cancel/expiry lifecycle against an injected clock.
//
// Safety posture: DTD and entity declarations are forbidden (Go's encoding/xml
// already rejects entity references, but a bare DOCTYPE is tolerated, so it is
// rejected explicitly). Operational (Actual/Public) alerts are distinguished
// from Exercise/Test/System/Draft data; only operational alerts may drive
// guidance, and none do here — this is the P2 ingestion/preview slice.
package capfeed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"sthira/backend/internal/contracts"
)

// MaxXMLBytes bounds a single CAP artifact.
const MaxXMLBytes = 1 << 20 // 1 MiB

// CAPError is a parse/validation failure; the artifact is quarantined.
type CAPError struct{ Reason string }

func (e *CAPError) Error() string { return e.Reason }

func fail(format string, args ...any) *CAPError {
	return &CAPError{Reason: fmt.Sprintf(format, args...)}
}

// CAP message enums (CAP 1.2 values).
type Status string

const (
	StatusActual   Status = "Actual"
	StatusExercise Status = "Exercise"
	StatusSystem   Status = "System"
	StatusTest     Status = "Test"
	StatusDraft    Status = "Draft"
)

type MessageType string

const (
	MsgAlert  MessageType = "Alert"
	MsgUpdate MessageType = "Update"
	MsgCancel MessageType = "Cancel"
	MsgAck    MessageType = "Ack"
	MsgError  MessageType = "Error"
)

type Scope string

const (
	ScopePublic     Scope = "Public"
	ScopeRestricted Scope = "Restricted"
	ScopePrivate    Scope = "Private"
)

// LifecycleState is the alert lifecycle.
type LifecycleState string

const (
	StateActive     LifecycleState = "ACTIVE"
	StateSuperseded LifecycleState = "SUPERSEDED"
	StateCancelled  LifecycleState = "CANCELLED"
	StateExpired    LifecycleState = "EXPIRED"
)

// Alert is the parsed, validated CAP alert. All text is inert data.
type Alert struct {
	Identifier string
	Sender     string
	Sent       time.Time
	Status     Status
	MsgType    MessageType
	Scope      Scope
	References []string
	Language   string
	Event      string
	Effective  time.Time
	Expires    time.Time
	Headline   string
	// Areas holds validated polygons in GeoJSON [longitude,latitude] order,
	// converted from CAP's latitude,longitude pairs.
	Areas [][]contracts.Point
	// Operational reports whether this is Actual+Public data. Exercise/Test/
	// System/Draft or non-Public alerts are never operational.
	Operational bool
}

// Parsed is a validated alert plus its preserved raw artifact and digest.
type Parsed struct {
	Alert  Alert
	RawXML []byte
	SHA256 string
	State  LifecycleState
}

// ParseOptions carries source identity and the injected clock.
type ParseOptions struct {
	SourceURI       string
	SenderAllowList map[string]bool
	// RetrievedAt is the ingestion instant (injected clock). Required.
	RetrievedAt time.Time
}

// internal XML decode shapes (namespace-agnostic local names).
type xmlAlert struct {
	Identifier string    `xml:"identifier"`
	Sender     string    `xml:"sender"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	Scope      string    `xml:"scope"`
	References string    `xml:"references"`
	Infos      []xmlInfo `xml:"info"`
}

type xmlInfo struct {
	Language    string    `xml:"language"`
	Category    string    `xml:"category"`
	Event       string    `xml:"event"`
	Effective   string    `xml:"effective"`
	Expires     string    `xml:"expires"`
	Headline    string    `xml:"headline"`
	Description string    `xml:"description"`
	Areas       []xmlArea `xml:"area"`
}

type xmlArea struct {
	AreaDesc string   `xml:"areaDesc"`
	Polygons []string `xml:"polygon"`
}

// Parse validates a CAP artifact. It never follows references, fetches URIs or
// evaluates content; the raw bytes are preserved verbatim for audit.
func Parse(rawXML []byte, opts ParseOptions) (*Parsed, error) {
	if len(rawXML) == 0 {
		return nil, fail("empty CAP artifact")
	}
	if int64(len(rawXML)) > MaxXMLBytes {
		return nil, fail("CAP artifact exceeds maximum size")
	}
	if opts.RetrievedAt.IsZero() {
		return nil, fail("retrieved_at (injected clock) is required")
	}
	lower := bytes.ToLower(rawXML)
	if bytes.Contains(lower, []byte("<!doctype")) || bytes.Contains(lower, []byte("<!entity")) {
		return nil, fail("DTD and entity declarations are forbidden")
	}

	var doc xmlAlert
	dec := xml.NewDecoder(bytes.NewReader(rawXML))
	if err := dec.Decode(&doc); err != nil {
		return nil, fail("malformed CAP XML: %v", err)
	}
	if err := ensureSingleRoot(dec); err != nil {
		return nil, err
	}

	identifier, err := required("identifier", doc.Identifier)
	if err != nil {
		return nil, err
	}
	sender, err := required("sender", doc.Sender)
	if err != nil {
		return nil, err
	}
	if len(opts.SenderAllowList) > 0 && !opts.SenderAllowList[sender] {
		return nil, fail("CAP sender %q is not allow-listed", sender)
	}
	sent, err := parseTime("sent", doc.Sent)
	if err != nil {
		return nil, err
	}
	status, err := parseStatus(doc.Status)
	if err != nil {
		return nil, err
	}
	msgType, err := parseMsgType(doc.MsgType)
	if err != nil {
		return nil, err
	}
	scope, err := parseScope(doc.Scope)
	if err != nil {
		return nil, err
	}
	references := strings.Fields(doc.References)
	if (msgType == MsgUpdate || msgType == MsgCancel) && len(references) == 0 {
		return nil, fail("CAP update/cancel requires references")
	}
	if len(doc.Infos) == 0 {
		return nil, fail("CAP requires an info block")
	}
	info := doc.Infos[0]

	effective := sent
	if strings.TrimSpace(info.Effective) != "" {
		effective, err = parseTime("effective", info.Effective)
		if err != nil {
			return nil, err
		}
	}
	expires, err := parseTime("expires", info.Expires)
	if err != nil {
		return nil, err
	}
	if !expires.After(effective) {
		return nil, fail("CAP expires must be after effective")
	}
	if len(info.Areas) == 0 {
		return nil, fail("CAP requires an area")
	}
	areas := make([][]contracts.Point, 0, len(info.Areas))
	for _, a := range info.Areas {
		for _, p := range a.Polygons {
			ring, err := parsePolygon(p)
			if err != nil {
				return nil, err
			}
			areas = append(areas, ring)
		}
	}

	sum := sha256.Sum256(rawXML)
	alert := Alert{
		Identifier:  identifier,
		Sender:      sender,
		Sent:        sent,
		Status:      status,
		MsgType:     msgType,
		Scope:       scope,
		References:  references,
		Language:    strings.TrimSpace(info.Language),
		Event:       strings.TrimSpace(info.Event),
		Effective:   effective,
		Expires:     expires,
		Headline:    strings.TrimSpace(info.Headline),
		Areas:       areas,
		Operational: status == StatusActual && scope == ScopePublic,
	}
	return &Parsed{
		Alert:  alert,
		RawXML: append([]byte(nil), rawXML...),
		SHA256: hex.EncodeToString(sum[:]),
		State:  StateActive,
	}, nil
}

func ensureSingleRoot(dec *xml.Decoder) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fail("malformed CAP XML: %v", err)
		}
		if _, ok := tok.(xml.StartElement); ok {
			return fail("unexpected content after CAP root element")
		}
	}
}

func required(field, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fail("missing or empty CAP field: %s", field)
	}
	return v, nil
}

// parseTime requires an RFC 3339 timestamp with an explicit offset and
// normalizes to UTC.
func parseTime(field, value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, fail("missing CAP timestamp: %s", field)
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, fail("invalid CAP timestamp %s: %q", field, value)
	}
	return t.UTC(), nil
}

// parsePolygon converts a CAP polygon (space-separated "lat,lon" pairs) into a
// closed GeoJSON ring of [longitude,latitude] points.
func parsePolygon(value string) ([]contracts.Point, error) {
	fields := strings.Fields(value)
	if len(fields) < 4 {
		return nil, fail("CAP polygon must have at least four positions")
	}
	ring := make([]contracts.Point, 0, len(fields))
	for _, pair := range fields {
		parts := strings.Split(pair, ",")
		if len(parts) != 2 {
			return nil, fail("invalid CAP polygon position %q", pair)
		}
		var lat, lon float64
		if _, err := fmt.Sscanf(parts[0], "%g", &lat); err != nil {
			return nil, fail("invalid CAP polygon latitude %q", parts[0])
		}
		if _, err := fmt.Sscanf(parts[1], "%g", &lon); err != nil {
			return nil, fail("invalid CAP polygon longitude %q", parts[1])
		}
		pt, err := contracts.NewPoint(lon, lat)
		if err != nil {
			return nil, fail("CAP polygon %v", err)
		}
		ring = append(ring, pt)
	}
	if ring[0] != ring[len(ring)-1] {
		return nil, fail("CAP polygon must be closed")
	}
	return ring, nil
}

func parseStatus(v string) (Status, error) {
	switch Status(strings.TrimSpace(v)) {
	case StatusActual, StatusExercise, StatusSystem, StatusTest, StatusDraft:
		return Status(strings.TrimSpace(v)), nil
	}
	return "", fail("invalid CAP status: %q", v)
}

func parseMsgType(v string) (MessageType, error) {
	switch MessageType(strings.TrimSpace(v)) {
	case MsgAlert, MsgUpdate, MsgCancel, MsgAck, MsgError:
		return MessageType(strings.TrimSpace(v)), nil
	}
	return "", fail("invalid CAP msgType: %q", v)
}

func parseScope(v string) (Scope, error) {
	switch Scope(strings.TrimSpace(v)) {
	case ScopePublic, ScopeRestricted, ScopePrivate:
		return Scope(strings.TrimSpace(v)), nil
	}
	return "", fail("invalid CAP scope: %q", v)
}
