package cmd

import (
	"errors"
	"fmt"
	"proxygw/pkg/ipc"
	"proxygw/pkg/ipc/method"
)

type Client struct {
	*ipc.BaseClient
}

func NewClient(conn *ipc.Conn) *Client {
	return &Client{
		BaseClient: ipc.NewBaseClient(conn),
	}
}

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

func (c *Client) Close() error {
	return c.BaseClient.Close()
}
