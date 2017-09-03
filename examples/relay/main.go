package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	net "github.com/nimona/go-nimona-net"
)

const (
	protocolID = "dummy"
	eventType  = "echo-message"
)

var (
	wg = &sync.WaitGroup{}

	addr = flag.String("addr", ":2180", "echo service address")
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	nrPort := 21700
	nrPeerID := "nr"
	nrAddr := "nr/dummy"

	n1Port := 21600
	n1PeerID := "n1"
	n1Addr := "n1/dummy"

	n2Port := 21610
	n2PeerID := "n2"
	n2Addr := "n2/dummy"

	// create networks
	// newNode will return a peer and a network
	pr, _, err := newNodeR(nrPort, nrPeerID)
	if err != nil {
		log.Fatal("Could not create n1", err)
	}
	p1, n1, err := newNode1(n1Port, n1PeerID)
	if err != nil {
		log.Fatal("Could not create n1", err)
	}
	p2, n2, err := newNode2(n2Port, n2PeerID)
	if err != nil {
		log.Fatal("Could not create n2", err)
	}

	// we now need to let each network now of the other peer
	// this is not required if we register a dht or other discovery method

	// add peers
	if err := n1.PutPeer(*pr); err != nil {
		log.Fatal("Could not add pr to n1")
	}
	if err := n1.PutPeer(*p2); err != nil {
		log.Fatal("Could not add p2 to n1")
	}

	// add peers
	if err := n2.PutPeer(*pr); err != nil {
		log.Fatal("Could not add pr to n1")
	}
	if err := n2.PutPeer(*p1); err != nil {
		log.Fatal("Could not add p1 to n2")
	}

	// wait a bit for everything to settle
	// time.Sleep(100 * time.Millisecond)

	fmt.Println(">>>", 1)
	// dial to relay
	if _, err := n1.Dial(nrAddr); err != nil {
		log.Fatal("Could not connect to relay", err)
	}

	// time.Sleep(time.Second * 2)

	fmt.Println(">>>", 2)
	if _, err := n2.Dial(nrAddr); err != nil {
		log.Fatal("Could not connect to relay", err)
	}

	// time.Sleep(time.Second * 2)

	fmt.Println(">>>", 3, n2Addr)
	// create a new stream from p1 to p2
	n1s, err := n1.Dial(n2Addr)
	if err != nil {
		log.Fatal("Could not create stream", err)
	}
	fmt.Println(">>>", 3.1, n2Addr)

	// time.Sleep(time.Second)
	// time.Sleep(time.Second)

	fmt.Println("Writing from p1 to p2")
	if _, err := n1s.Write([]byte("Hello from p1!\n")); err != nil {
		log.Fatal("Could not write to n1s", err)
	}
	wg.Add(1)

	// time.Sleep(time.Second * 2)

	fmt.Println(">>>", 4)
	// create a new stream from p2 to p1
	n1s22, err := n2.Dial(n1Addr)
	if err != nil {
		log.Fatal("Could not create stream", err)
	}
	fmt.Println("Writing from p2 to p1")
	if _, err := n1s22.Write([]byte("Hello back from p2!\n")); err != nil {
		log.Fatal("Could not write to n1s", err)
	}
	wg.Add(1)

	fmt.Println(">>>", 5)
	// create a new stream from p2 to p1
	n1s33, err := n2.Dial(n1Addr)
	if err != nil {
		log.Fatal("Could not create stream", err)
	}
	fmt.Println("Writing from p2 to p1 AGAIN!!!!!!")
	if _, err := n1s33.Write([]byte("Hello back from p2!!!!!!!!!!!!\n")); err != nil {
		log.Fatal("Could not write to n1s", err)
	}
	wg.Add(1)

	// wait a bit to receive both messages
	wg.Wait()
}

// func newNode(port int, peerID, relayed string) (*net.Peer, net.Network, error) {
// 	// create local peer
// 	oaddrs, _ := net.GetAddresses(port)
// 	addrs := []string{}
// 	if relayed == "" {
// 		addrs, _ = net.GetAddresses(port)
// 	} else {
// 		addrs = append(addrs, "relay:nr/"+peerID)
// 	}
// 	// for i, addr := range addrs {
// 	// 	addrs[i] = relayed + addr
// 	// }
// 	// if relayed != "" {
// 	// 	for _, addr := range addrs {
// 	// 		addrs = append(addrs, relayed+addr)
// 	// 	}
// 	// }
// 	pr := &net.Peer{
// 		ID:        peerID,
// 		Addresses: addrs,
// 	}

// 	// initialize network
// 	mn, err := net.NewNetwork(pr)
// 	if err != nil {
// 		fmt.Println("Could not initialize network", err)
// 		return nil, nil, err
// 	}

// 	for _, oaddr := range oaddrs {
// 		mn.Listen(oaddr)
// 	}

// 	// create a stream handler
// 	hn := func(protocolID string, rwc io.ReadWriteCloser) error {
// 		scanner := bufio.NewScanner(rwc)
// 		for scanner.Scan() {
// 			fmt.Printf("* Received text in peer=%s, text=%s\n", pr.ID, scanner.Text())
// 			wg.Done()
// 		}
// 		if err := scanner.Err(); err != nil {
// 			fmt.Fprintln(os.Stderr, "reading standard input:", err)
// 		}
// 		return nil
// 	}

// 	// handle incomming events
// 	if err := mn.RegisterStreamHandler(protocolID, hn); err != nil {
// 		fmt.Println("Could not attach stream handler", err)
// 		return nil, nil, err
// 	}

// 	// print some info
// 	ip := "127.0.0.1"
// 	fmt.Printf("New node: host=%s:%d id=%s\n", ip, port, peerID)

// 	return pr, mn, nil
// }

func newNode1(port int, peerID string) (*net.Peer, net.Network, error) {
	// create local peer
	oaddrs, _ := net.GetAddresses(port)
	pr := &net.Peer{
		ID: peerID,
		Addresses: []string{
			"relay:nr/n1",
		},
	}

	// initialize network
	mn, err := net.NewNetwork(pr, port)
	if err != nil {
		fmt.Println("Could not initialize network", err)
		return nil, nil, err
	}

	for _, oaddr := range oaddrs {
		mn.Listen(oaddr)
	}

	// create a stream handler
	hn := func(protocolID string, rwc io.ReadWriteCloser) error {
		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			fmt.Printf("* Received text in peer=%s, text=%s\n", pr.ID, scanner.Text())
			wg.Done()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return nil
	}

	// handle incomming events
	if err := mn.RegisterStreamHandler(protocolID, hn); err != nil {
		fmt.Println("Could not attach stream handler", err)
		return nil, nil, err
	}

	// print some info
	ip := "127.0.0.1"
	fmt.Printf("New node: host=%s:%d id=%s\n", ip, port, peerID)

	return pr, mn, nil
}

func newNode2(port int, peerID string) (*net.Peer, net.Network, error) {
	// create local peer
	oaddrs, _ := net.GetAddresses(port)
	pr := &net.Peer{
		ID: peerID,
		Addresses: []string{
			"relay:nr/n2",
		},
	}

	// initialize network
	mn, err := net.NewNetwork(pr, port)
	if err != nil {
		fmt.Println("Could not initialize network", err)
		return nil, nil, err
	}

	for _, oaddr := range oaddrs {
		mn.Listen(oaddr)
	}

	// create a stream handler
	hn := func(protocolID string, rwc io.ReadWriteCloser) error {
		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			fmt.Printf("* Received text in peer=%s, text=%s\n", pr.ID, scanner.Text())
			wg.Done()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return nil
	}

	// handle incomming events
	if err := mn.RegisterStreamHandler(protocolID, hn); err != nil {
		fmt.Println("Could not attach stream handler", err)
		return nil, nil, err
	}

	// print some info
	ip := "127.0.0.1"
	fmt.Printf("New node: host=%s:%d id=%s\n", ip, port, peerID)

	return pr, mn, nil
}

func newNodeR(port int, peerID string) (*net.Peer, net.Network, error) {
	// create local peer
	oaddrs, _ := net.GetAddresses(port)
	pr := &net.Peer{
		ID:        peerID,
		Addresses: oaddrs,
	}

	// initialize network
	mn, err := net.NewNetwork(pr, port)
	if err != nil {
		fmt.Println("Could not initialize network", err)
		return nil, nil, err
	}

	for _, oaddr := range oaddrs {
		mn.Listen(oaddr)
	}

	// create a stream handler
	hn := func(protocolID string, rwc io.ReadWriteCloser) error {
		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			fmt.Printf("* Received text in peer=%s, text=%s\n", pr.ID, scanner.Text())
			wg.Done()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		return nil
	}

	// handle incomming events
	if err := mn.RegisterStreamHandler(protocolID, hn); err != nil {
		fmt.Println("Could not attach stream handler", err)
		return nil, nil, err
	}

	// print some info
	ip := "127.0.0.1"
	fmt.Printf("New node: host=%s:%d id=%s\n", ip, port, peerID)

	return pr, mn, nil
}
