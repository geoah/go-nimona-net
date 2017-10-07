package net

import (
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	// ErrorNotFound is returned when peer does not exist in PeerStore
	ErrorNotFound = errors.New("Peer not found")
)

// peerstore is thread safe in-memory implementation of Peerstore
type peerstore struct {
	mutex    sync.RWMutex
	peers    map[string]Peer
	handlers []func(Peer) error
}

func (ps *peerstore) Put(peer Peer) error {
	ps.mutex.Lock()
	if ep, ok := ps.peers[peer.ID]; ok {
		for _, addr := range peer.Addresses {
			exists := false
			for _, eaddr := range ep.Addresses {
				if eaddr == addr {
					exists = true
					break
				}
			}
			if !exists {
				ep.Addresses = append(ep.Addresses, addr)
			}
		}
		ps.peers[peer.ID] = ep
	} else {
		ps.peers[peer.ID] = peer
	}
	logrus.WithField("pid", peer.ID).WithField("addrs", ps.peers[peer.ID].Addresses).Infof("Updated peer info")
	ps.mutex.Unlock()
	ps.notifyPut(peer)
	return nil
}

func (ps *peerstore) Remove(id string) error {
	// TODO Set alive false, put, notify
	// ps.mutex.Lock()
	// delete(ps.peers, id)
	// ps.mutex.Unlock()
	// ps.notifyPut(id)
	return nil
}

func (ps *peerstore) Get(id string) (Peer, error) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	peer, ok := ps.peers[id]
	if ok == false {
		return Peer{}, ErrorNotFound
	}
	return peer, nil
}

func (ps *peerstore) Peers() []Peer {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	peers := make([]Peer, len(ps.peers))
	i := 0
	for _, peer := range ps.peers {
		peers[i] = peer
		i++
	}
	return peers
}

func (ps *peerstore) RegisterPeerHandler(handler func(Peer) error) error {
	ps.handlers = append(ps.handlers, handler)
	return nil
}

func (ps *peerstore) notifyPut(peer Peer) error {
	for _, handler := range ps.handlers {
		handler(peer)
	}
	return nil
}

// NewPeerstore returns an empty peerstore
func NewPeerstore() *peerstore {
	return &peerstore{
		peers: map[string]Peer{},
	}
}
