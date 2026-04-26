package ipc

import (
	"net"
	"proxygw/plugin/ctl/ipc/codec"
	"time"
)

type Conn struct {
	conn  net.Conn
	codec codec.Codec
	enc   codec.Encoder
	dec   codec.Decoder
}

func Dial(network string, address string, codec codec.Codec) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return WrapConn(conn, codec), nil
}

func WrapConn(conn net.Conn, codec codec.Codec) *Conn {
	enc := codec.NewEncoder(conn)
	dec := codec.NewDecoder(conn)
	result := &Conn{conn, codec, enc, dec}
	return result
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) Codec() codec.Codec {
	return c.codec
}

func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *Conn) Write(p *Packet) error {
	return c.enc.Encode(p)
}

func (c *Conn) Read() (Packet, error) {
	p := Packet{0, 0, c.codec.Raw()}
	err := c.dec.Decode(&p)
	return p, err
}

func (c *Conn) ReadTo(p *Packet) error {
	err := c.dec.Decode(p)
	return err
}

func (c *Conn) Close() error {
	return c.conn.Close()
}
