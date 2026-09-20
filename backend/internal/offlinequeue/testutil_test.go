package offlinequeue

import "os"

// readFileOS is a tiny wrapper so test files can read queue state without
// pulling os into every file directly.
func readFileOS(path string) ([]byte, error) { return os.ReadFile(path) }
