package net

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

const (
	maxDialTimeoutSeconds = 10
)

// TCPTransport -
type TCPTransport struct {
}

// NewTCPTransport -
func NewTCPTransport() Transport {
	return &TCPTransport{}
}

// Dial -
func (t *TCPTransport) Dial(addr string) (net.Conn, error) {
	return t.DialContext(context.Background(), addr)
}

// DialContext -
func (t *TCPTransport) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	if t.matches(addr) == false {
		return nil, ErrTransportNotSupported
	}

	caddr, err := t.getCleanAddr(addr)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{Timeout: time.Second * maxDialTimeoutSeconds}
	c, err := d.DialContext(ctx, "tcp", caddr)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Listen -
func (t *TCPTransport) Listen(addr string) (net.Listener, error) {
	if t.matches(addr) == false {
		return nil, ErrTransportNotSupported
	}

	caddr, err := t.getCleanAddr(addr)
	if err != nil {
		return nil, err
	}

	return net.Listen("tcp", caddr)
}

func (t *TCPTransport) matches(addr string) bool {
	pr := strings.Split(addr, ":")[0]
	if pr == "tcp" || pr == "tcp4" || pr == "tcp6" {
		return true
	}
	return false
}

func (t *TCPTransport) getCleanAddr(addr string) (string, error) {
	pa := strings.Split(addr, "/")[0]
	pr := strings.Split(pa, ":")
	if len(pr) < 2 {
		return "", errors.New("Invalid address")
	}
	return strings.Join(pr[1:], ":"), nil
}
