package cli

import (
	"proxygw/pkg/ipc"
)

type Client struct {
	conn   *ipc.Conn
	nextId uint32
}

func NewClient(conn *ipc.Conn) *Client {
	return &Client{conn, 0}
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) Call(method uint16, req any, res any) error {
	id := c.nextId
	c.nextId++

	p := method.Packet{id, method, req}
	if err := c.conn.Write(&p); err != nil {
		return err
	}

	p.Body = res
	if err := c.conn.ReadTo(&p); err != nil {
		return err
	}

	return nil
}
