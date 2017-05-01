package net

import (
	"errors"
	"fmt"
	"io"
	"net"

	ms "github.com/multiformats/go-multistream"

	mux "github.com/nimona/go-nimona-mux"
)

const (
	dummyProtocolID = "/nimona/dummy"
)

var (
	// ErrorCNF Could not get address from peer id
	ErrorCNF = errors.New("Could not resolve peer ID")
)

type HandlerFunc func(proto string, rwc io.ReadWriteCloser) error

// NewTCPNetwork -
func NewTCPNetwork(peer Peer) (*TCPNetwork, error) {
	net := &TCPNetwork{
		peer:         peer,
		peers:        map[string]Peer{},
		multiplexers: map[string]*mux.Mux{},
		mux:  ms.NewMultistreamMuxer(),
		cmux: ms.NewMultistreamMuxer(),
	}
	net.cmux.AddHandler("/cselect/v1.0.0", net.handleConnection)
	go net.handle()
	return net, nil
}

// TCPNetwork is the simplest possible network
type TCPNetwork struct {
	// TODO sync.Mutex for dials maybe?
	peer         Peer
	peers        map[string]Peer
	multiplexers map[string]*mux.Mux
	mux  *ms.MultistreamMuxer
	cmux *ms.MultistreamMuxer
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

	err = ms.SelectProtoOrFail("/cselect/v1.0.0", c)
	if err != nil {
		fmt.Println("SELERR1", err)
		return nil, err
	}

	mss, err := mux.New(c)
	if err != nil {
		fmt.Println("MUXNERR", err)
		return nil, err
	}

	// TODO Should we also start handling this protocol?
	// eg. go n.handlers[protocolID](ms)

	n.multiplexers[peerID] = mss

	st, err := mss.NewStream()
	if err != nil {
		return nil, err
	}

	err = ms.SelectProtoOrFail(protocolID, st)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// HandleStream incoming streams
func (n *TCPNetwork) HandleStream(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
	// TODO protocols are not supported yet as we need a selection protocol (the irony)
	protocolID = dummyProtocolID
	if _, ok := n.handlers[protocolID]; ok {
		// TODO Is this really needed?
		return errors.New("Protocol already registered")
	n.mux.AddHandler(protocolID, handler)
	return nil
}

func (n *TCPNetwork) handleConnection(proto string, rwc io.ReadWriteCloser) error {
	fmt.Println("Handling connection")
	ms, err := mux.New(rwc)
	if err != nil {
		return err
	}

	for {
		mss, err := ms.Accept() // TODO Handle error
		if err != nil {
			continue
		}

		go n.mux.Handle(mss)
	}
	return nil
}

// handle incoming events
func (n *TCPNetwork) handle() error {
	c, err := net.Listen("tcp", n.peer.GetAddresses()[0])
	if err != nil {
		return err
	}

	for {
		ss, err := c.Accept()
		if err != nil {
			continue
		}
		go n.cmux.Handle(ss)
	}
}
