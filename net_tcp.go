package net

import (
	"errors"
	"fmt"
	"net"

	mux "github.com/nimona/go-nimona-mux"
)

const (
	dummyProtocolID = "/nimona/dummy"
)

var (
	// ErrorCNF Could not get address from peer id
	ErrorCNF = errors.New("Could not resolve peer ID")
)

// NewTCPNetwork -
func NewTCPNetwork(peer Peer) (*TCPNetwork, error) {
	net := &TCPNetwork{
		peer:         peer,
		peers:        map[string]Peer{},
		multiplexers: map[string]*mux.Mux{},
	}
	go net.handle()
	return net, nil
}

// TCPNetwork is the simplest possible network
type TCPNetwork struct {
	// TODO sync.Mutex for dials maybe?
	peer         Peer
	peers        map[string]Peer
	multiplexers map[string]*mux.Mux
	handlers     map[string]func(stream *mux.Stream)
}

// GetPeers -
func (n *TCPNetwork) GetPeers() map[string]Peer {
	return n.peers
}

// Add peer to network
func (n *TCPNetwork) Add(peer Peer) error {
	if n.peers == nil {
		n.peers = map[string]Peer{}
	}
	n.peers[peer.GetID()] = peer
	return nil
}

// NewStream -
func (n *TCPNetwork) NewStream(protocolID, peerID string) (*mux.Stream, error) {
	if n.multiplexers == nil {
		n.multiplexers = map[string]*mux.Mux{}
	}

	if ms, ok := n.multiplexers[peerID]; ok {
		if str, err := ms.NewStream(); err == nil {
			return str, nil
		}
	}

	peer, ok := n.peers[peerID]
	if !ok {
		return nil, ErrorCNF // TODO Better error
	}

	c, err := net.Dial("tcp", peer.GetAddresses()[0])
	if err != nil {
		return nil, err
	}

	ms, err := mux.New(c)
	if err != nil {
		return nil, err
	}

	// TODO Should we also start handling this protocol?
	// eg. go n.handlers[protocolID](ms)

	n.multiplexers[peerID] = ms

	return ms.NewStream()
}

// Handle incoming events
func (n *TCPNetwork) Handle(protocolID string, handler func(stream *mux.Stream)) error {
	// TODO protocols are not supported yet as we need a selection protocol (the irony)
	protocolID = dummyProtocolID
	if _, ok := n.handlers[protocolID]; ok {
		// TODO Is this really needed?
		return errors.New("Protocol already registered")
	}

	n.handlers[protocolID] = handler
	return nil
}

// handle incoming events
func (n *TCPNetwork) handle() error {
	c, err := net.Listen("tcp", n.peer.GetAddresses()[0])
	if err != nil {
		return err
	}

	ss, _ := c.Accept()
	ms, err := mux.New(ss)
	if err != nil {
		return err
	}

	for {
		mss, _ := ms.Accept() // TODO Handle error
		fmt.Println("*** Accepted connection")
		// TODO protocols are not yet supported
		// TODO at some point we need to read the protocol and find the right handler
	}
}
