package ipc

import (
	"net"
	"proxygw/pkg/ipc/codec"
)

// Listener implements a base listener type for accepting IPC peers
type Listener struct {
	listener net.Listener
	codec    codec.Codec
}

func Listen(network string, address string, codec codec.Codec) (*Listener, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return WrapListener(listener, codec), nil
}

func WrapListener(listener net.Listener, codec codec.Codec) *Listener {
	return &Listener{listener, codec}
}

func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *Listener) Codec() codec.Codec {
	return l.codec
}

func (l *Listener) Accept() (*Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return WrapConn(conn, l.codec), nil
}

func (l *Listener) Close() error {
	return l.listener.Close()
}
