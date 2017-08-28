package net

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

// Relay -
type Relay struct {
	net Network
}

func (r *Relay) handleNewStream(protocolID string, rwc io.ReadWriteCloser) error {
	logrus.Infof("New relay stream with protocolID " + protocolID)
	// defer rwc.Close()

	reader := bufio.NewReader(rwc)
	taddr, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	taddr = strings.Trim(taddr, "\n")

	// dial target
	c, err := r.net.Dial(taddr)
	if err != nil {
		logrus.
			WithField("taddr", taddr).
			WithError(err).
			Warnf("Could not dial peer")
		return err
	}

	go func() {
		if err := Pipe(c, rwc); err != nil {
			logrus.
				WithError(err).
				Errorf("Could not start pipe")
		}
	}()

	rwc.Write([]byte("ok\n"))

	return nil
}

// Dial -
func (r *Relay) Dial(addr string) (net.Conn, error) {
	return r.DialContext(context.Background(), addr)
}

// DialContext -
func (r *Relay) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	if r.matches(addr) == false {
		return nil, ErrTransportNotSupported
	}

	raddr, err := r.getRelayAddr(addr)
	if err != nil {
		return nil, err
	}

	taddr, err := r.getTargetAddr(addr)
	if err != nil {
		return nil, err
	}

	raddr = raddr + "/relay"

	logrus.
		WithField("addr", addr).
		WithField("raddr", raddr).
		WithField("taddr", taddr).
		Warnf("Dialing target peer")

	c, err := r.net.Dial(raddr)
	if err != nil {
		return nil, err
	}

	c.Write([]byte(taddr + "\n"))

	reader := bufio.NewReader(c)
	if _, err := reader.ReadString('\n'); err != nil {
		return nil, err
	}

	return c, nil
}

// Listen -
func (r *Relay) Listen(addr string) (net.Listener, error) {
	return nil, errors.New("Not implemented")
}

func (r *Relay) matches(addr string) bool {
	pr := strings.Split(addr, ":")[0]
	if pr == "relay" {
		return true
	}
	return false
}

func (r *Relay) getRelayAddr(addr string) (string, error) {
	pa := strings.Split(addr, "/")[0]
	pr := strings.Split(pa, ":")
	if len(pr) < 2 {
		return "", errors.New("Invalid address")
	}
	return strings.Join(pr[1:], ":"), nil
}

func (r *Relay) getTargetAddr(addr string) (string, error) {
	pa := strings.Split(addr, "/")
	if len(pa) < 2 {
		return "", errors.New("Invalid address")
	}
	return strings.Join(pa[1:], "/"), nil
}

func Pipe(a, b io.ReadWriteCloser) error {
	done := make(chan error, 1)
	cp := func(r, w io.ReadWriteCloser) {
		n, err := io.Copy(r, w)
		logrus.Infof("copied %d bytes", n)
		done <- err
	}
	go cp(a, b)
	go cp(b, a)
	return <-done
}
