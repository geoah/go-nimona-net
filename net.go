package net

import mux "github.com/nimona/go-nimona-mux"

// Network -
type Network interface {
	HandleSteam(protocolID string, handler func(stream *mux.Stream) error) error
	NewStream(protocolID, peerID string) (*mux.Stream, error)
}
