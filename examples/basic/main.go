package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	net "github.com/nimona/go-nimona-net"
)

const (
	protocolID = "/dummy/v0"
	eventType  = "echo-message"
)

var addr = flag.String("addr", ":2180", "echo service address")

func main() {
	n1Port := 21600
	n1PeerID := "n1"

	n2Port := 21610
	n2PeerID := "n2"

	// create networks
	// newNode will return a peer and a network
	p1, n1, err := newNode(n1Port, n1PeerID)
	if err != nil {
		log.Fatal("Could not create n1", err)
	}
	p2, n2, err := newNode(n2Port, n2PeerID)
	if err != nil {
		log.Fatal("Could not create n2", err)
	}

	// we now need to let each network now of the other peer
	// this is not required if we register a dht or other discovery method

	// add p2 to n1
	if err := n1.PutPeer(*p2); err != nil {
		log.Fatal("Could not add p2 to n1")
	}

	// add p1 to n2
	if err := n2.PutPeer(*p1); err != nil {
		log.Fatal("Could not add p1 to n2")
	}

	// wait a bit for everything to settle
	time.Sleep(100 * time.Millisecond)

	// create a new stream from p1 to p2
	n1s, err := n1.NewStream(protocolID, p2.ID)
	if err != nil {
		log.Fatal("Could not create stream", err)
	}
	fmt.Println("Writing from p1 to p2")
	if _, err := n1s.Write([]byte("Hello from p1!\n")); err != nil {
		log.Fatal("Could not write to n1s", err)
	}

	// create a new stream from p2 to p1
	n1s, err = n2.NewStream(protocolID, p1.ID)
	if err != nil {
		log.Fatal("Could not create stream", err)
	}
	fmt.Println("Writing from p2 to p1")
	if _, err := n1s.Write([]byte("Hello back from p2!\n")); err != nil {
		log.Fatal("Could not write to n1s", err)
	}

	// wait a bit to receive both messages
	time.Sleep(1 * time.Second)
}

func newNode(port int, peerID string) (*net.Peer, net.Network, error) {
	// create local peer
	host := fmt.Sprintf("127.0.0.1:%d", port)
	pr := &net.Peer{
		ID:        peerID,
		Addresses: []string{host},
	}

	// initialize network
	mn, err := net.NewTCPNetwork(pr)
	if err != nil {
		fmt.Println("Could not initialize network", err)
		return nil, nil, err
	}

	// create a stream handler
	hn := func(protocolID string, rwc io.ReadWriteCloser) error {
		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			fmt.Printf("* Received text in peer=%s, text=%s\n", pr.ID, scanner.Text())
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
