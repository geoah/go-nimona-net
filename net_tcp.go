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
	smux "github.com/xtaci/smux"
)

const (
	SmuxProtocolID = "/smux/v1"
)

// NewTCPNetwork
func NewTCPNetwork(peer *Peer, port int) (Network, error) {
	if port == 0 {
		port = getPort()
	}

	net := &TCPNetwork{
		peer:         peer,
		port:         port,
		peerstore:    NewPeerstore(),
		multiplexers: map[string]*smux.Session{},
		mux:          ms.NewMultistreamMuxer(),
		cmux:         ms.NewMultistreamMuxer(),
	}
	net.cmux.AddHandler(SmuxProtocolID, net.handleConnection)

	go net.handle()
	// go net.getPublicAddress()
	// go net.mapPorts()

	return net, nil
}

// TCPNetwork is the simplest possible network
type TCPNetwork struct {
	// TODO sync.Mutex for dials maybe?
	port         int
	peer         *Peer
	peerstore    *peerstore
	multiplexers map[string]*smux.Session
	mux          *ms.MultistreamMuxer
	cmux         *ms.MultistreamMuxer
}

// NewStream
func (n *TCPNetwork) NewStream(protocolID, peerID string) (*smux.Stream, error) {
	if n.multiplexers == nil {
		n.multiplexers = map[string]*smux.Session{}
	}

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

	addr := ""
	for _, iaddr := range peer.Addresses {
		prv, err := privateIP(iaddr)
		if err == nil && prv == true {
			continue
		}
		addr = iaddr
	}

	if addr == "" {
		return nil, errors.New("No valid ip")
	}

	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
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

// RegisterStreamHandler for incoming streams
func (n *TCPNetwork) RegisterStreamHandler(protocolID string, handler func(proto string, stream io.ReadWriteCloser) error) error {
	n.mux.AddHandler(protocolID, handler)
	return nil
}

func (n *TCPNetwork) handleConnection(proto string, rwc io.ReadWriteCloser) error {
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
			if addr == "" {
				continue
			}
			hst := strings.Split(addr, ":")[0]
			prv, err := privateIP(hst)
			if err == nil && prv == true {
				continue
			} else {
				logrus.WithField("ip", ip).
					WithField("prv", prv).
					WithError(err).
					Debugf("Appending IP")
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
	c, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", n.port))
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

// Ask the kernel for a free open port that is ready to use
func getPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func privateIP(ip string) (bool, error) {
	var err error
	private := false
	// IP := net.ParseIP(ip)
	// if IP == nil {
	// 	err = errors.New("Invalid IP")
	// } else if IP.String() == "0.0.0.0" {
	// 	private = true
	// } else if IP.String() == "127.0.0.1" {
	// 	private = true
	// } else {
	// 	_, private24BitBlock, _ := net.ParseCIDR("10.0.0.0/8")
	// 	_, private20BitBlock, _ := net.ParseCIDR("172.16.0.0/12")
	// 	_, private16BitBlock, _ := net.ParseCIDR("192.168.0.0/16")
	// 	private = private24BitBlock.Contains(IP) ||
	// 		private20BitBlock.Contains(IP) ||
	// 		private16BitBlock.Contains(IP)
	// }
	return private, err
}
