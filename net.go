package net

import (
	"io"

	"github.com/xtaci/smux"
)

type PeerEvent string

const (
	PeerEventCreated PeerEvent = "CREATED"
	PeerEventUpdated           = "UPDATED"
	PeerEventRemoved           = "REMOVED"
)

// Network -
type Network interface {
	// NewStream creates a new stram with a specific protocol
	NewStream(protocolID, peerID string) (*smux.Stream, error)
	// RegisterStreamHandler adds a stream handler for a specific protocol
	RegisterStreamHandler(protocolID string, handler func(protocolID string, rwc io.ReadWriteCloser) error) error

	// GetLocalPeer retuns local peer
	GetLocalPeer() (*Peer, error)
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
