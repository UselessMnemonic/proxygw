package main

import (
	"errors"
	"fmt"
	"proxygw/plugin/ctl/ipc"
	"proxygw/plugin/ctl/ipc/method"
)

// Client is a small typed wrapper around the control IPC client.
type Client struct {
	*ipc.BaseClient
}

// NewClient creates a control client over conn.
func NewClient(conn *ipc.Conn) *Client {
	return &Client{
		BaseClient: ipc.NewBaseClient(conn),
	}
}

// StatusRequest fetches the daemon's current target and frontend status.
func (c *Client) StatusRequest() (*method.StatusResponse, error) {
	ch, err := c.Request(&ipc.Packet{
		Method: method.MethodStatusRequest,
		Body:   method.StatusRequest{},
	})
	if err != nil {
		return nil, err
	}

	resp, ok := <-ch
	if !ok {
		return nil, errors.New("ipc connection closed")
	}

	switch body := resp.Body.(type) {
	case *method.StatusResponse:
		return body, nil
	case *method.ErrorResponse:
		return nil, body
	default:
		return nil, fmt.Errorf("unexpected response body type %T for method %d", resp.Body, resp.Method)
	}
}

// Close closes the underlying IPC connection.
func (c *Client) Close() error {
	return c.BaseClient.Close()
}
