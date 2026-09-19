package capfeed

import (
	"strings"
	"testing"
	"time"
)

// fixedNow is the deterministic injected clock for parser tests.
var fixedNow = time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

func allowAll() map[string]bool { return nil }

func allowList(senders ...string) map[string]bool {
	m := make(map[string]bool, len(senders))
	for _, s := range senders {
		m[s] = true
	}
	return m
}

// capXML builds a well-formed CAP alert with overridable fields. Tests mutate
// the returned string to create malformed variants.
type capFields struct {
	Identifier string
	Sender     string
	Sent       string
	Status     string
	MsgType    string
	Scope      string
	References string
	Expires    string
	Polygon    string
}

func baseFields() capFields {
	return capFields{
		Identifier: "ALERT-1",
		Sender:     "synthetic.ndma.example",
		Sent:       "2026-09-12T04:00:00Z",
		Status:     "Actual",
		MsgType:    "Alert",
		Scope:      "Public",
		Expires:    "2026-09-13T04:00:00Z",
		Polygon:    "11.45,76.00 11.67,76.24 11.50,76.24 11.45,76.00",
	}
}

func (f capFields) xml() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2">` + "\n")
	b.WriteString("  <identifier>" + f.Identifier + "</identifier>\n")
	b.WriteString("  <sender>" + f.Sender + "</sender>\n")
	b.WriteString("  <sent>" + f.Sent + "</sent>\n")
	b.WriteString("  <status>" + f.Status + "</status>\n")
	b.WriteString("  <msgType>" + f.MsgType + "</msgType>\n")
	b.WriteString("  <scope>" + f.Scope + "</scope>\n")
	if f.References != "" {
		b.WriteString("  <references>" + f.References + "</references>\n")
	}
	b.WriteString("  <info>\n")
	b.WriteString("    <language>en-IN</language>\n")
	b.WriteString("    <category>Geo</category>\n")
	b.WriteString("    <event>Synthetic flood exercise</event>\n")
	b.WriteString("    <effective>" + f.Sent + "</effective>\n")
	b.WriteString("    <expires>" + f.Expires + "</expires>\n")
	b.WriteString("    <headline>SYNTHETIC DEMO</headline>\n")
	b.WriteString("    <area>\n")
	b.WriteString("      <areaDesc>Synthetic demo area</areaDesc>\n")
	b.WriteString("      <polygon>" + f.Polygon + "</polygon>\n")
	b.WriteString("    </area>\n")
	b.WriteString("  </info>\n")
	b.WriteString("</alert>\n")
	return b.String()
}

func parseOK(t *testing.T, xml string, opts ParseOptions) *Parsed {
	t.Helper()
	p, err := Parse([]byte(xml), opts)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return p
}

func parseErr(t *testing.T, xml string, opts ParseOptions) error {
	t.Helper()
	_, err := Parse([]byte(xml), opts)
	if err == nil {
		t.Fatalf("Parse succeeded, want error")
	}
	return err
}

func TestParseValidOperational(t *testing.T) {
	p := parseOK(t, baseFields().xml(), ParseOptions{
		SourceURI:       "fixture:cap",
		SenderAllowList: allowList("synthetic.ndma.example"),
		RetrievedAt:     fixedNow,
	})
	if p.Alert.Identifier != "ALERT-1" {
		t.Errorf("identifier = %q", p.Alert.Identifier)
	}
	if !p.Alert.Operational {
		t.Error("Actual+Public should be operational")
	}
	if p.State != StateActive {
		t.Errorf("state = %q", p.State)
	}
	if len(p.SHA256) != 64 {
		t.Errorf("sha256 length = %d", len(p.SHA256))
	}
	if len(p.RawXML) == 0 {
		t.Error("raw XML not preserved")
	}
	// Polygon conversion: CAP lat,lon -> GeoJSON lon,lat.
	if len(p.Alert.Areas) != 1 {
		t.Fatalf("areas = %d", len(p.Alert.Areas))
	}
	ring := p.Alert.Areas[0]
	if len(ring) != 4 {
		t.Fatalf("ring positions = %d", len(ring))
	}
	first := ring[0].Coordinates
	// CAP "11.45,76.00" (lat,lon) must become [76.00, 11.45] (lon,lat).
	if first[0] != 76.00 || first[1] != 11.45 {
		t.Errorf("first point = %v, want [76 11.45]", first)
	}
}

func TestParseExerciseNotOperational(t *testing.T) {
	f := baseFields()
	f.Status = "Exercise"
	p := parseOK(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
	if p.Alert.Operational {
		t.Error("Exercise must not be operational")
	}
}

func TestParseRejectsDOCTYPE(t *testing.T) {
	xml := strings.Replace(baseFields().xml(), "<alert", "<!DOCTYPE alert><alert", 1)
	err := parseErr(t, xml, ParseOptions{RetrievedAt: fixedNow})
	if !strings.Contains(err.Error(), "DTD") {
		t.Errorf("err = %v, want DTD rejection", err)
	}
}

func TestParseRejectsEntity(t *testing.T) {
	xml := strings.Replace(baseFields().xml(), "<alert", "<!ENTITY x \"y\"><alert", 1)
	parseErr(t, xml, ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsMalformed(t *testing.T) {
	parseErr(t, "<alert><identifier>x</identifier>", ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsEmpty(t *testing.T) {
	parseErr(t, "", ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsOversized(t *testing.T) {
	big := strings.Repeat("x", MaxXMLBytes+1)
	parseErr(t, big, ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRequiresClock(t *testing.T) {
	err := parseErr(t, baseFields().xml(), ParseOptions{})
	if !strings.Contains(err.Error(), "retrieved_at") {
		t.Errorf("err = %v, want clock requirement", err)
	}
}

func TestParseSenderAllowList(t *testing.T) {
	err := parseErr(t, baseFields().xml(), ParseOptions{
		SenderAllowList: allowList("other.sender"),
		RetrievedAt:     fixedNow,
	})
	if !strings.Contains(err.Error(), "not allow-listed") {
		t.Errorf("err = %v, want allow-list rejection", err)
	}
}

func TestParseUpdateRequiresReferences(t *testing.T) {
	f := baseFields()
	f.MsgType = "Update"
	f.References = ""
	parseErr(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsExpiresBeforeEffective(t *testing.T) {
	f := baseFields()
	f.Expires = "2026-09-11T04:00:00Z" // before effective/sent
	parseErr(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsOpenPolygon(t *testing.T) {
	f := baseFields()
	f.Polygon = "11.45,76.00 11.67,76.24 11.50,76.24" // not closed
	parseErr(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsOutOfRangeCoordinate(t *testing.T) {
	f := baseFields()
	f.Polygon = "95.00,76.00 11.67,76.24 11.50,76.24 95.00,76.00" // lat 95 invalid
	parseErr(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
}

func TestParseRejectsInvalidStatus(t *testing.T) {
	f := baseFields()
	f.Status = "Bogus"
	parseErr(t, f.xml(), ParseOptions{RetrievedAt: fixedNow})
}
