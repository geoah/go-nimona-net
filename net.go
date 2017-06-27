package net

import (
	"io"

	mux "github.com/nimona/go-nimona-mux"
)

// Network -
type Network interface {
	// NewStream creates a new stram with a specific protocol
	NewStream(protocolID, peerID string) (*mux.Stream, error)
	// RegisterStreamHandler adds a stream handler for a specific protocol
	RegisterStreamHandler(protocolID string, handler func(protocolID string, rwc io.ReadWriteCloser) error) error

	// PutPeer adds or updates a Peer
	PutPeer(peer Peer) error
	// RemovePeer a Peer
	RemovePeer(id string) error
	// GetPeer retrieves a Peer by its ID
	GetPeer(id string) (Peer, error)
	// GetPeers returns all Peers in this peer store
	GetPeers() []Peer
	// RegisterPeerHandler can register multiple handlers that listen for peer updates
	RegisterPeerHandler(func(Peer) error) error
}
