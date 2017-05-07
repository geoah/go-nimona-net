package net

import (
	"io"
	"net"

	ms "github.com/multiformats/go-multistream"

	mux "github.com/nimona/go-nimona-mux"
	ps "github.com/nimona/go-nimona-peerstore"
)

// HandlerFunc
type HandlerFunc func(proto string, rwc io.ReadWriteCloser) error

// NewTCPNetwork
func NewTCPNetwork(peer ps.Peer, peerstore ps.Peerstore) (*TCPNetwork, error) {
	net := &TCPNetwork{
		peer:         peer,
		peerstore:    peerstore,
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
	peer         ps.Peer
	peerstore    ps.Peerstore
	multiplexers map[string]*mux.Mux
	mux          *ms.MultistreamMuxer
	cmux         *ms.MultistreamMuxer
}

// NewStream
func (n *TCPNetwork) NewStream(protocolID, peerID string) (*mux.Stream, error) {
	if n.multiplexers == nil {
		n.multiplexers = map[string]*mux.Mux{}
	}

	if ms, ok := n.multiplexers[peerID]; ok {
		if str, err := ms.NewStream(); err == nil {
			return str, nil
		}
	}

	peer, err := n.peerstore.Get(ps.ID(peerID))
	if err != nil {
		return nil, err
	}

	c, err := net.Dial("tcp", peer.GetAddresses()[0]) // TODO Find the correct address
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

// HandleStream incoming streams
func (n *TCPNetwork) HandleStream(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
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
