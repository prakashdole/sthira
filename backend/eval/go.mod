module sthira/backend/eval

go 1.27

// The eval conformance suite exercises the ACTUAL worker HTTP server
// constructors (asrworker/middleworker/ttsworker) with fake
// runtimes, so the provider wire shapes are proven against the real
// handlers, not hand-written stand-ins. These are the same-module
// siblings resolved by filesystem replace so no published version
// exists.
require (
	sthira/backend/internal/asrworker v0.0.0
	sthira/backend/internal/middleworker v0.0.0
	sthira/backend/internal/ttsworker v0.0.0
)

replace sthira/backend/internal/asrworker => ../internal/asrworker

replace sthira/backend/internal/middleworker => ../internal/middleworker

replace sthira/backend/internal/ttsworker => ../internal/ttsworker
