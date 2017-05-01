package net

import (
	"io"

	mux "github.com/nimona/go-nimona-mux"
)

// Network -
type Network interface {
	HandleStream(protocolID string, handler func(protocolID string, rwc io.ReadWriteCloser) error) error
	NewStream(protocolID, peerID string) (*mux.Stream, error)
}
