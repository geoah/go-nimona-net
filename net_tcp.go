package net

import (
	"io"
	"net"

	ms "github.com/multiformats/go-multistream"

	mux "github.com/nimona/go-nimona-mux"
)

// NewTCPNetwork
func NewTCPNetwork(peer *Peer) (Network, error) {
	net := &TCPNetwork{
		peer:         peer,
		peerstore:    NewPeerstore(),
		multiplexers: map[string]*mux.Mux{},
		mux:          ms.NewMultistreamMuxer(),
		cmux:         ms.NewMultistreamMuxer(),
	}
	net.cmux.AddHandler(mux.ProtocolID, net.handleConnection)
	go net.handle()
	return net, nil
}

// TCPNetwork is the simplest possible network
type TCPNetwork struct {
	// TODO sync.Mutex for dials maybe?
	peer         *Peer
	peerstore    *peerstore
	multiplexers map[string]*mux.Mux
	mux          *ms.MultistreamMuxer
	cmux         *ms.MultistreamMuxer
}

// NewStream
func (n *TCPNetwork) NewStream(protocolID, peerID string) (*mux.Stream, error) {
	if n.multiplexers == nil {
		n.multiplexers = map[string]*mux.Mux{}
	}

	if mss, ok := n.multiplexers[peerID]; ok {
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

	peer, err := n.peerstore.Get(peerID)
	if err != nil {
		return nil, err
	}

	c, err := net.Dial("tcp", peer.Addresses[0]) // TODO Find the correct address
	if err != nil {
		return nil, err
	}

	err = ms.SelectProtoOrFail(mux.ProtocolID, c)
	if err != nil {
		return nil, err
	}

	mss, err := mux.New(c)
	if err != nil {
		return nil, err
	}

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

// RegisterStreamHandler for incoming streams
func (n *TCPNetwork) RegisterStreamHandler(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
	n.mux.AddHandler(protocolID, handler)
	return nil
}

func (n *TCPNetwork) handleConnection(proto string, rwc io.ReadWriteCloser) error {
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
}

// handle incoming events
func (n *TCPNetwork) handle() error {
	// TODO Pick correct address
	c, err := net.Listen("tcp", n.peer.Addresses[0])
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

// PutPeer adds or updates a Peer
func (n *TCPNetwork) PutPeer(peer Peer) error {
	return n.peerstore.Put(peer)
}

// RemovePeer a Peer
func (n *TCPNetwork) RemovePeer(id string) error {
	return n.peerstore.Remove(id)
}

// GetPeer retrieves a Peer by its ID
func (n *TCPNetwork) GetPeer(id string) (Peer, error) {
	return n.peerstore.Get(id)
}

// GetPeers returns all Peers in this peer store
func (n *TCPNetwork) GetPeers() []Peer {
	return n.peerstore.Peers()
}

// RegisterPeerHandler can register multiple handlers that listen for peer updates
func (n *TCPNetwork) RegisterPeerHandler(handler func(Peer) error) error {
	return n.peerstore.RegisterPeerHandler(handler)
}
