package ttsworker

import (
	"bufio"
	"io"
)

// newStdScanner wraps a bufio.Scanner over an io.Reader. The
// returned lineScanner satisfies the dispatcher's minimal
// interface so a fake lineScanner can be substituted in tests.
func newStdScanner(rd io.Reader, maxLine int) lineScanner {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	return sc
}
