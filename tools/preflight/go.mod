// Module preflight is a standalone, dependency-free local launch-readiness
// checker for the real-model worker procedure (asrworker, middleworker,
// ttsworker). Isolated under its own go.mod so it never affects backend's
// dependency graph and can be built/run independently of the product
// binaries it inspects. Standard library only.
module sthira/tools/preflight

go 1.23
