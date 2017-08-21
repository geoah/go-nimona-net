package net

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"

	ms "github.com/multiformats/go-multistream"
	logrus "github.com/sirupsen/logrus"
	smux "github.com/xtaci/smux"
)

const (
	// SmuxProtocolID -
	SmuxProtocolID = "/smux/v1"
)

var (
	// ErrTransportNotSupported -
	ErrTransportNotSupported = errors.New("Transport not supported")
)

// Network -
type Network interface {
	// AddTransport -
	AddTransport(transport Transport) error
	// Dial will figure out a peer's addresses and connect to it
	Dial(addr string) (net.Conn, error)
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

// NewNetwork -
func NewNetwork(peer *Peer) (Network, error) {
	net := &network{
		transports: []Transport{
			NewTCPTransport(),
		},
		listeners:    []net.Listener{},
		connections:  map[string]net.Conn{},
		peer:         peer,
		peerstore:    NewPeerstore(),
		multiplexers: map[string]*smux.Session{},
		mux:          ms.NewMultistreamMuxer(),
		cmux:         ms.NewMultistreamMuxer(),
	}
	net.cmux.AddHandler(SmuxProtocolID, net.handleConnection)

	for _, addr := range net.peer.Addresses {
		for _, tr := range net.transports {
			go func(addr string, tr Transport) {
				c, err := tr.Listen(addr)
				if err != nil {
					logrus.
						WithField("addr", addr).
						WithField("transport", reflect.TypeOf(tr)).
						WithError(err).
						Warnf("Could not listen to transport")
					return
				}
				logrus.
					WithField("addr", addr).
					WithField("tr", reflect.TypeOf(tr)).
					Infof("Started listening")
					// start accepting connections
				for {
					ss, err := c.Accept()
					if err != nil {
						continue
					}
					go net.cmux.Handle(ss)
				}
			}(addr, tr)
		}
	}

	return net, nil
}

// network is the simplest possible network
type network struct {
	sync.Mutex   // used for both dialing and adding transports
	transports   []Transport
	connections  map[string]net.Conn
	listeners    []net.Listener
	peer         *Peer
	peerstore    *peerstore
	multiplexers map[string]*smux.Session
	mux          *ms.MultistreamMuxer
	cmux         *ms.MultistreamMuxer
}

// Dial -
func (n *network) Dial(addr string) (net.Conn, error) {
	return n.DialWithContext(context.Background(), addr)
}

// DialWithContext -
func (n *network) DialWithContext(ctx context.Context, addr string) (net.Conn, error) {
	ap := strings.Split(addr, "/")
	if len(ap) < 2 {
		return nil, errors.New("Missing protocol")
	}

	peerID := ap[0]
	protocolID := strings.Join(ap[1:], "/")

	logrus.
		WithField("peerID", peerID).
		WithField("procotolID", protocolID).
		Debugf("Dialing peer")

	if mss, ok := n.multiplexers[peerID]; ok {
		st, err := mss.OpenStream()
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

	if len(peer.Addresses) == 0 {
		return nil, errors.New("Peer has no addresses")
	}

	var c net.Conn

ConnectionLoop:
	for _, addr := range peer.Addresses {
		for _, tr := range n.transports {
			var err error
			c, err = tr.DialContext(ctx, addr)
			if err != nil {
				logrus.
					WithField("addr", addr).
					WithField("transport", reflect.TypeOf(tr)).
					WithError(err).
					Debugf("Could not dial transport")
			}
			break ConnectionLoop
		}
	}
	if c == nil {
		logrus.Debugf("All transports failed")
		return nil, ErrTransportNotSupported
	}

	err = ms.SelectProtoOrFail(SmuxProtocolID, c)
	if err != nil {
		return nil, err
	}

	mss, err := smux.Client(c, nil)
	if err != nil {
		return nil, err
	}

	n.multiplexers[peerID] = mss

	st, err := mss.OpenStream()
	if err != nil {
		return nil, err
	}

	err = ms.SelectProtoOrFail(protocolID, st)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (n *network) AddTransport(tr Transport) error {

	return nil
}

// RegisterStreamHandler for incoming streams
func (n *network) RegisterStreamHandler(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
	n.mux.AddHandler(protocolID, handler)
	return nil
}

func (n *network) handleConnection(proto string, rwc io.ReadWriteCloser) error {
	ms, err := smux.Server(rwc, nil)
	if err != nil {
		return err
	}

	for {
		mss, err := ms.AcceptStream() // TODO Handle error
		if err != nil {
			continue
		}

		go n.mux.Handle(mss)
	}
}

// PutPeer adds or updates a Peer
func (n *network) PutPeer(peer Peer) error {
	return n.peerstore.Put(peer)
}

// RemovePeer a Peer
func (n *network) RemovePeer(id string) error {
	return n.peerstore.Remove(id)
}

// GetLocalPeer retrieves the local peer
func (n *network) GetLocalPeer() (*Peer, error) {
	return n.peer, nil
}

// GetPeer retrieves a Peer by its ID
func (n *network) GetPeer(id string) (Peer, error) {
	return n.peerstore.Get(id)
}

// GetPeers returns all Peers in this peer store
func (n *network) GetPeers() []Peer {
	return n.peerstore.Peers()
}

// RegisterPeerHandler can register multiple handlers that listen for peer updates
func (n *network) RegisterPeerHandler(handler func(Peer) error) error {
	return n.peerstore.RegisterPeerHandler(handler)
}
