package net

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"

	// smux "github.com/hashicorp/yamux"
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
	// Dial will figure out a peer's addresses and connect to it
	Dial(addr string) (net.Conn, error)
	// Listen -
	Listen(addr string) (net.Listener, error)

	// AddTransport -
	AddTransport(transport Transport) error
	// RegisterStreamHandler adds a stream handler for a specific protocol
	RegisterStreamHandler(protocolID string, handler func(protocolID string, rwc io.ReadWriteCloser) error) error

	// GetLocalPeer retuns local peer
	GetLocalPeer() *Peer
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
func NewNetwork(peer *Peer, port int) (Network, error) {
	n := &network{
		transports: []Transport{
			NewTCPTransport(),
		},
		listeners: []net.Listener{},
		peer:      peer,
		peerstore: NewPeerstore(),
		sessions:  map[string]*smux.Session{},
		mux:       ms.NewMultistreamMuxer(),
		cmux:      ms.NewMultistreamMuxer(),
		// ids:          map[string]chan string{},
	}

	n.cmux.AddHandler(SmuxProtocolID, n.handleConnection)

	if len(peer.Addresses) == 0 {
		if port == 0 {
			port = GetPort()
		}
		addrs, _ := GetAddresses(port)
		peer.Addresses = addrs
	}

	for _, addr := range n.peer.Addresses {
		n.Listen(addr)
	}

	relay := &Relay{net: n}
	n.mux.AddHandler("relay", relay.handleNewStream)
	n.AddTransport(relay)

	return n, nil
}

// network is the simplest possible network
type network struct {
	sync.Mutex // used for both dialing and adding transports
	transports []Transport
	listeners  []net.Listener
	peer       *Peer
	peerstore  *peerstore
	sessions   map[string]*smux.Session
	mux        *ms.MultistreamMuxer
	cmux       *ms.MultistreamMuxer
	// ids          map[string]chan string // map[nonce]remotePeerID
}

// Dial -
func (n *network) Dial(addr string) (net.Conn, error) {
	return n.DialWithContext(context.Background(), addr)
}

// TODO Used for debugging, remove asap.
var DDIAL int

// DialWithContext -
func (n *network) DialWithContext(ctx context.Context, addr string) (net.Conn, error) {
	DDIAL++
	dial := DDIAL

	ap := strings.Split(addr, "/")
	if len(ap) < 2 {
		return nil, errors.New("Missing protocol")
	}

	tpid := ap[0] // target peer id
	protocolID := strings.Join(ap[1:], "/")

	if tpid == n.GetLocalPeer().ID {
		return nil, errors.New("I'm not dialing myself")
	}

	logger := logrus.
		WithField("lpid", n.GetLocalPeer().ID).
		WithField("tpid", tpid).
		WithField("DIAL", dial).
		WithField("procotolID", protocolID)

	logger.Debugf("Dialing peer")

	if mss, ok := n.sessions[tpid]; ok {
		if mss.IsClosed() {
			logrus.Errorf("Session is closed, dialing again")
		} else {
			logger.Infof("Found existing peer ms")
			st, err := mss.OpenStream()
			if err != nil {
				return nil, err
			}
			logger.Infof("Selecting protocol")
			err = ms.SelectProtoOrFail(protocolID, st)
			if err != nil {
				return nil, err
			}
			logger.Infof("Dialing complete, used existing session")
			return st, nil
		}
	}

	peer, err := n.peerstore.Get(tpid)
	if err != nil {
		return nil, err
	}

	if len(peer.Addresses) == 0 {
		return nil, errors.New("Peer has no addresses")
	}

	var c net.Conn
	var utr Transport
	var daddr string

ConnectionLoop:
	// try to connect to an address
	for _, raddr := range peer.Addresses {
		iraddr := raddr + "/" + protocolID
		// with any available protocol
		for _, tr := range n.transports {
			logger.
				WithField("tranport", reflect.TypeOf(tr)).
				Infof("Dialing peer with transport")
			var err error
			c, err = tr.DialContext(ctx, iraddr)
			if err != nil {
				continue
			}
			utr = tr
			daddr = iraddr
			logger.
				WithField("transport", reflect.TypeOf(utr)).
				Infof("Dialed")
			// stop once a connection was establised
			break ConnectionLoop
		}
	}
	// else just return
	if c == nil {
		logger.Debugf("All transports failed")
		return nil, ErrTransportNotSupported
	}

	logger = logger.
		WithField("transport", reflect.TypeOf(utr)).
		WithField("daddr", daddr)

	// if we connected through a relay, we can't have multiplexed streams
	// on top of the already muliplexed streams
	// simply return the stream and the two ends will then handle protocol
	// selection on their own
	// TODO This is a very ugly hack, fixing this requires refactoring Dial
	if _, ok := utr.(*Relay); ok {
		logger.Debugf("Dialing complete, was relayed")
		return c, nil
	}

	logger.Debugf("Selecting session protocol")

	// select the multiplexer protocol
	err = ms.SelectProtoOrFail(SmuxProtocolID, c)
	if err != nil {
		return nil, err
	}

	logger.Debugf("Sending local peer id")

	// inform of the other end our peer id
	// this will allow the other party to re-use the already established
	// connection when it needs one, instead of trying to dial a new one
	// TODO move this to an indetify protocol or something
	c.Write([]byte(n.GetLocalPeer().ID + "\n"))
	mss, err := smux.Server(c, nil)
	if err != nil {
		logrus.
			WithError(err).
			Warnf("Could not init server-side mux")
		return nil, err
	}

	n.sessions[tpid] = mss

	logger.Debugf("Accepting streams")

	// start accepting streams on the muliplexed connection
	go func(imss *smux.Session) {
		for {
			// wait until the other side opens a new stream
			mssa, err := imss.AcceptStream() // TODO Handle error
			if err != nil {
				continue
			}
			// once a stream has been accepted, we should handle the selected
			// protocol
			go n.mux.Handle(mssa)
		}
	}(mss)

	logger.Debugf("Opening stream")

	// open new stream
	st, err := mss.OpenStream()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Selecting stream protocol")

	// select protocol
	err = ms.SelectProtoOrFail(protocolID, st)
	if err != nil {
		logger.
			WithError(err).
			Infof("Could not stream select protocol")
		return nil, err
	}

	logger.Infof("Dialing complete")

	return st, nil
}

// Listen -
func (n *network) Listen(addr string) (net.Listener, error) {
	for _, tr := range n.transports {
		lst, err := tr.Listen(addr)
		if err != nil {
			logrus.
				WithField("addr", addr).
				WithField("transport", reflect.TypeOf(tr)).
				WithError(err).
				Warnf("Could not listen to transport")
			continue
		}
		logrus.
			WithField("addr", addr).
			WithField("tr", reflect.TypeOf(tr)).
			Infof("Started listening")

		// start accepting connections
		go func(lst net.Listener) {
			for {
				ss, err := lst.Accept()
				if err != nil {
					// TODO Log/Handle error
					continue
				}
				go n.cmux.Handle(ss)
			}
		}(lst)
	}
	// TODO Implement common listener?
	return nil, nil
}

func (n *network) AddTransport(tr Transport) error {
	n.transports = append(n.transports, tr)
	return nil
}

// RegisterStreamHandler for incoming streams
func (n *network) RegisterStreamHandler(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
	n.mux.AddHandler(protocolID, handler)
	return nil
}

func (n *network) handleConnection(proto string, rwc io.ReadWriteCloser) error {
	// move to an identity protocol
	reader := bufio.NewReader(rwc)
	pid, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	pid = strings.Trim(pid, "\n")
	logrus.
		WithField("lpid", n.GetLocalPeer().ID).
		WithField("rpid", pid).
		Errorf("Got remote peer id")

	msc, err := smux.Client(rwc, nil)
	if err != nil {
		logrus.
			WithError(err).
			Warnf("Could not init client-side mux")
		return err
	}

	n.sessions[pid] = msc

	logrus.Infof("Accepting mux streams")
	go func(imsc *smux.Session) {
		for {
			mss, err := imsc.AcceptStream()
			if err != nil {
				logrus.WithError(err).Warnf("Could not accept stream")
				imsc.Close()
				return
			}
			logrus.Infof("Accepted mux stream")
			go n.mux.Handle(mss)
		}
	}(msc)

	return nil
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
func (n *network) GetLocalPeer() *Peer {
	return n.peer
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
