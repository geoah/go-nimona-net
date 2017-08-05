package net

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	ms "github.com/multiformats/go-multistream"
	upnp "github.com/prestonTao/upnp"
	logrus "github.com/sirupsen/logrus"

	mux "github.com/nimona/go-nimona-mux"
)

// NewTCPNetwork
func NewTCPNetwork(peer *Peer, port int) (Network, error) {
	net := &TCPNetwork{
		peer:         peer,
		port:         port,
		peerstore:    NewPeerstore(),
		multiplexers: map[string]*mux.Mux{},
		mux:          ms.NewMultistreamMuxer(),
		cmux:         ms.NewMultistreamMuxer(),
	}
	net.cmux.AddHandler(mux.ProtocolID, net.handleConnection)

	go net.handle()
	go net.getPublicAddress()
	go net.mapPorts()

	return net, nil
}

// TCPNetwork is the simplest possible network
type TCPNetwork struct {
	// TODO sync.Mutex for dials maybe?
	port         int
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

	if len(peer.Addresses) == 0 {
		return nil, errors.New("Peer has no addresses")
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
	// add all addresses to peer
	n.peer.Addresses = []string{}

	// go through all ifs
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	// and find their addresses
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			addr := ""
			if strings.Contains(ip.String(), ":") {
				addr = fmt.Sprintf("[%s]:%d", ip.String(), n.port)
			} else {
				addr = fmt.Sprintf("%s:%d", ip.String(), n.port)
			}
			if addr != "" {
				n.peer.Addresses = append(n.peer.Addresses, addr)
				fmt.Printf("> Adding address %s\n", addr)
			}
		}
	}

	// broadcast local peer
	if err := n.PutPeer(*n.peer); err != nil {
		logrus.WithError(err).Warningf("Coud not put local peer")
	}

	// listen to all interfaces
	c, err := net.Listen("tcp", fmt.Sprintf(":%d", n.port))
	if err != nil {
		return err
	}

	// start accepting connections
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

// GetLocalPeer retrieves the local peer
func (n *TCPNetwork) GetLocalPeer() (*Peer, error) {
	return n.peer, nil
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

func (n *TCPNetwork) getPublicAddress() {
	// TODO Better logging
	upnpMan := new(upnp.Upnp)
	err := upnpMan.ExternalIPAddr()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// add public address to local peer
	naddr := fmt.Sprintf("%s:%d", upnpMan.GatewayOutsideIP, n.port)
	n.peer.Addresses = append(n.peer.Addresses, naddr)

	// broadcast local peer
	if err := n.PutPeer(*n.peer); err != nil {
		logrus.WithError(err).Warningf("Coud not put local peer")
	}
}

func (n *TCPNetwork) mapPorts() {
	mapping := new(upnp.Upnp)
	if err := mapping.AddPortMapping(n.port, n.port, "TCP"); err == nil {
		fmt.Println("success mapping port !")
		// TODO Need to reclaim port at some point
		// mapping.Reclaim()
	} else {
		fmt.Println("fail !")
	}
}
