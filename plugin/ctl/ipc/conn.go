package ipc

import (
	"net"
	"time"

	"github.com/UselessMnemonic/proxygw/plugin/ctl/ipc/codec"
)

// Conn wraps a net.Conn with packet encoding and decoding.
type Conn struct {
	conn  net.Conn
	codec codec.Codec
	enc   codec.Encoder
	dec   codec.Decoder
}

// Dial opens an IPC connection using codec for packet serialization.
func Dial(network string, address string, codec codec.Codec) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return WrapConn(conn, codec), nil
}

// WrapConn adapts an existing net.Conn for packet IPC.
func WrapConn(conn net.Conn, codec codec.Codec) *Conn {
	enc := codec.NewEncoder(conn)
	dec := codec.NewDecoder(conn)
	result := &Conn{conn, codec, enc, dec}
	return result
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Codec returns the serialization codec used by this connection.
func (c *Conn) Codec() codec.Codec {
	return c.codec
}

// SetDeadline sets the read and write deadlines on the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Write sends one packet.
func (c *Conn) Write(p *Packet) error {
	return c.enc.Encode(p)
}

// Read receives one packet and leaves its body as codec-specific raw bytes.
func (c *Conn) Read() (Packet, error) {
	p := Packet{0, 0, c.codec.Raw()}
	err := c.dec.Decode(&p)
	p.Body = c.codec.UnwrapRaw(p.Body)
	return p, err
}

// ReadTo receives one packet into p.
func (c *Conn) ReadTo(p *Packet) error {
	err := c.dec.Decode(p)
	return err
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}
