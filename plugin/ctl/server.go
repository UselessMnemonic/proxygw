package ctl

import (
	"context"
	"fmt"
	"net"
	"slices"

	"github.com/UselessMnemonic/proxygw/pkg/engine"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
	"github.com/UselessMnemonic/proxygw/pkg/target"
	"github.com/UselessMnemonic/proxygw/plugin/ctl/ipc"
	"github.com/UselessMnemonic/proxygw/plugin/ctl/ipc/method"
)

// Server handles control IPC requests against an Engine.
type Server struct {
	*ipc.BaseServer
	engine *engine.Engine
}

// NewServer binds a control server to conn.
func NewServer(conn *ipc.Conn, engine *engine.Engine) *Server {
	if engine == nil {
		panic("engine must not be nil")
	}
	if conn == nil {
		panic("conn must not be nil")
	}
	s := &Server{
		BaseServer: ipc.NewBaseServer(conn),
		engine:     engine,
	}
	return s
}

// Serve processes requests until ctx is canceled or the connection closes.
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			s.BaseServer.Close()
			return net.ErrClosed
		case req, ok := <-s.Requests():
			if !ok {
				return net.ErrClosed
			}
			if err := s.handle(req); err != nil {
				return err
			}
		case <-s.Notifications():
			continue
		}
	}
}

func (s *Server) handle(req ipc.Packet) error {
	var resp ipc.Packet
	switch req.Body.(type) {
	case *method.StatusRequest:
		resp = ipc.MakePacket(req.Id, s.statusResponse())
	default:
		resp = ipc.MakePacket(req.Id, method.ErrorResponse{
			Message: fmt.Sprintf("unsupported method %d", req.Method),
		})
	}
	return s.Respond(resp)
}

func (s *Server) statusResponse() method.StatusResponse {
	resp := method.StatusResponse{
		Closed: s.engine.Closed(),
	}

	targets := s.engine.Targets()
	slices.SortFunc(targets, func(a, b *target.Target) int {
		switch {
		case a.Name() < b.Name():
			return -1
		case a.Name() > b.Name():
			return 1
		default:
			return 0
		}
	})
	resp.Targets = make([]method.TargetStatus, 0, len(targets))
	for _, t := range targets {
		targetStatus := method.TargetStatus{
			Name:  t.Name(),
			Kind:  t.Kind(),
			State: t.State().String(),
		}
		if err := t.Error(); err != nil {
			targetStatus.LastError = err.Error()
		}

		endpoints := t.Endpoints()
		slices.SortFunc(endpoints, func(a, b target.Endpoint) int {
			switch {
			case a.Name < b.Name:
				return -1
			case a.Name > b.Name:
				return 1
			default:
				return 0
			}
		})
		targetStatus.Endpoints = make([]method.TargetEndpointInfo, 0, len(endpoints))
		for _, ep := range endpoints {
			targetStatus.Endpoints = append(targetStatus.Endpoints, method.TargetEndpointInfo{
				Name:     ep.Name,
				Protocol: ep.Protocol,
				Address:  ep.Address.String(),
			})
		}

		resp.Targets = append(resp.Targets, targetStatus)
	}

	frontends := s.engine.Frontends()
	slices.SortFunc(frontends, func(a, b *frontend.Frontend) int {
		switch {
		case a.Name() < b.Name():
			return -1
		case a.Name() > b.Name():
			return 1
		default:
			return 0
		}
	})
	resp.Frontends = make([]method.FrontendStatus, 0, len(frontends))
	for _, f := range frontends {
		endpoint := f.Endpoint()
		frontendStatus := method.FrontendStatus{
			Name:         f.Name(),
			Kind:         f.Kind(),
			State:        f.State().String(),
			Protocol:     f.Protocol(),
			Listen:       f.Listen(),
			TargetName:   f.Target().Name(),
			EndpointName: endpoint.Name,
			ProxyAddress: f.ProxyAddress(),
		}
		if err := f.Error(); err != nil {
			frontendStatus.LastError = err.Error()
		}
		resp.Frontends = append(resp.Frontends, frontendStatus)
	}

	return resp
}
