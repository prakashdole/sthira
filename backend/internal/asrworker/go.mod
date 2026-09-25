// Module asrworker is the private ASR worker. It depends only on the
// Go standard library so worker safety review is independent of the
// orchestrator's dependency surface. The contract envelopes it speaks
// are read directly via contracts (no shared Go types imported across
// modules; the JSON wire format is the contract).
//
// The runtime adapter (Python or native) is wired in at startup via
// the Runtime interface; a real production deployment is required to
// provide a Runtime that satisfies the contract. See doc.go for the
// full retention and readiness rules.
module sthira/backend/internal/asrworker

go 1.23
