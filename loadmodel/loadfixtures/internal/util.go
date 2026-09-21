// Package internal holds small private helpers shared by the synthetic
// fixture server and the dummy P6 worker. It is internal to loadmodel so
// the public surface stays narrow.
package internal

import (
	"math/rand/v2"
	"net"
)

// RandFloat returns a uniformly random float in [0, 1).
func RandFloat() float64 { return rand.Float64() }

// NewListener parses addr and binds. addr=":0" picks an ephemeral port.
func NewListener(addr string) (net.Listener, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	return net.Listen("tcp", addr)
}
