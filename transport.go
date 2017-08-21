package net

import (
	"context"
	"net"
)

// Transport -
type Transport interface {
	Dial(addr string) (net.Conn, error)
	DialContext(ctx context.Context, addr string) (net.Conn, error)
	Listen(addr string) (net.Listener, error)
}
