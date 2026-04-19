package plugin

import (
	"proxygw/pkg/ipc"
)

type Client struct {
	conn *ipc.Conn
}

func NewClient(conn *ipc.Conn) *Client {
	return &Client{conn}
}

func (c *Client) Close() error {
	return c.conn.Close()
}
