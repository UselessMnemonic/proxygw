package ipc

import (
	"net"

	"github.com/UselessMnemonic/proxygw/plugin/ctl/ipc/codec"
)

// Listener implements a base listener type for accepting IPC peers
type Listener struct {
	listener net.Listener
	codec    codec.Codec
}

// Listen starts an IPC listener that wraps accepted connections with codec.
func Listen(network string, address string, codec codec.Codec) (*Listener, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return WrapListener(listener, codec), nil
}

// WrapListener adapts an existing net.Listener for packet IPC.
func WrapListener(listener net.Listener, codec codec.Codec) *Listener {
	return &Listener{listener, codec}
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// Codec returns the serialization codec used for accepted connections.
func (l *Listener) Codec() codec.Codec {
	return l.codec
}

// Accept waits for and wraps the next incoming connection.
func (l *Listener) Accept() (*Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return WrapConn(conn, l.codec), nil
}

// Close closes the underlying listener.
func (l *Listener) Close() error {
	return l.listener.Close()
}
