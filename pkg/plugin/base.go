package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"proxygw/pkg/ipc"
	"proxygw/pkg/ipc/codec"
	"proxygw/pkg/ipc/method"
	"slices"
	"sync"
	"syscall"
)

const pluginListenEnv = "PLUGIN_LISTEN"
const pluginListenNet = "unix"

type Base struct {
	*ipc.BaseServer

	lock             sync.RWMutex
	initHandler      InitHandler
	shutdownHandlers []ShutdownHandler
	targetHandlers   map[string]TargetHandler
	frontendHandlers map[string]FrontendHandler
	targetToKind     map[string]string
	frontendToKind   map[string]string
	closeOnce        sync.Once
}

func NewBase(wireCodec codec.Codec) (*Base, error) {
	if wireCodec == nil {
		return nil, errors.New("codec is nil")
	}

	address := os.Getenv(pluginListenEnv)
	if address == "" {
		return nil, fmt.Errorf("%s is not set", pluginListenEnv)
	}

	conn, err := ipc.Dial(pluginListenNet, address, wireCodec)
	if err != nil {
		return nil, fmt.Errorf("connect to plugin host: %w", err)
	}

	return &Base{
		BaseServer:       ipc.NewBaseServer(conn),
		targetHandlers:   make(map[string]TargetHandler),
		frontendHandlers: make(map[string]FrontendHandler),
		targetToKind:     make(map[string]string),
		frontendToKind:   make(map[string]string),
	}, nil
}

func (b *Base) HandleInit(handler InitHandler) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.initHandler = handler
}

func (b *Base) HandleShutdown(handler ShutdownHandler) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.shutdownHandlers = append(b.shutdownHandlers, handler)
}

func (b *Base) HandleTarget(kind string, handler TargetHandler) error {
	if kind == "" {
		return errors.New("target kind is empty")
	}
	if handler == nil {
		return errors.New("target handler is nil")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.targetHandlers[kind] = handler
	return nil
}

func (b *Base) HandleFrontend(kind string, handler FrontendHandler) error {
	if kind == "" {
		return errors.New("frontend kind is empty")
	}
	if handler == nil {
		return errors.New("frontend handler is nil")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.frontendHandlers[kind] = handler
	return nil
}

func (b *Base) NotifyFrontendShouldWarm(frontendID string) error {
	if frontendID == "" {
		return errors.New("frontend name is empty")
	}
	return b.Notify(&ipc.Packet{
		Method: method.MethodFrontendShouldWarmNotification,
		Body:   method.FrontendShouldWarmNotification{Name: frontendID},
	})
}

func (b *Base) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var loopErr error
	for loopErr == nil {
		select {
		case <-ctx.Done():
			return b.shutdown(context.Background(), nil)
		case req, ok := <-b.Requests():
			if !ok {
				return b.shutdown(context.Background(), nil)
			}
			loopErr = b.respond(req)
		case _, ok := <-b.Notifications():
			if !ok {
				return b.shutdown(context.Background(), nil)
			}
		}
	}

	return b.shutdown(context.Background(), loopErr)
}

func (b *Base) Close() error {
	var err error
	b.closeOnce.Do(func() {
		err = b.BaseServer.Close()
	})
	return err
}

func (b *Base) respond(req ipc.Packet) error {
	resp, err := b.handle(req)
	if err != nil {
		resp = ipc.MakePacket(req.Id, method.ErrorResponse{Message: err.Error()})
	}
	return b.Respond(resp)
}

func (b *Base) handle(req ipc.Packet) (ipc.Packet, error) {
	switch body := req.Body.(type) {
	case *method.PluginInitRequest:
		return b.handleInit(req.Id, body)
	case *method.NewTargetRequest:
		return b.handleNewTarget(req.Id, body)
	case *method.WarmTargetRequest:
		return b.handleWarmTarget(req.Id, body)
	case *method.DrainTargetRequest:
		return b.handleDrainTarget(req.Id, body)
	case *method.CloseTargetRequest:
		return b.handleCloseTarget(req.Id, body)
	case *method.NewFrontendRequest:
		return b.handleNewFrontend(req.Id, body)
	case *method.StartFrontendRequest:
		return b.handleStartFrontend(req.Id, body)
	case *method.StopFrontendRequest:
		return b.handleStopFrontend(req.Id, body)
	case *method.CloseFrontendRequest:
		return b.handleCloseFrontend(req.Id, body)
	default:
		return ipc.Packet{}, fmt.Errorf("unsupported method %d", req.Method)
	}
}

func (b *Base) handleInit(id uint32, req *method.PluginInitRequest) (ipc.Packet, error) {
	b.lock.RLock()
	handler := b.initHandler
	b.lock.RUnlock()
	if handler == nil {
		return ipc.MakePacket(id, b.initResponse()), nil
	}
	if err := handler(req.Options); err != nil {
		return ipc.Packet{}, err
	}
	return ipc.MakePacket(id, b.initResponse()), nil
}

func (b *Base) initResponse() method.PluginInitResponse {
	b.lock.RLock()
	defer b.lock.RUnlock()

	resp := method.PluginInitResponse{
		TargetKinds:   make([]string, 0, len(b.targetHandlers)),
		FrontendKinds: make([]string, 0, len(b.frontendHandlers)),
	}
	for name := range b.targetHandlers {
		resp.TargetKinds = append(resp.TargetKinds, name)
	}
	for name := range b.frontendHandlers {
		resp.FrontendKinds = append(resp.FrontendKinds, name)
	}
	slices.Sort(resp.TargetKinds)
	slices.Sort(resp.FrontendKinds)
	return resp
}

func (b *Base) handleNewTarget(id uint32, req *method.NewTargetRequest) (ipc.Packet, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, exists := b.targetToKind[req.Name]; exists {
		return ipc.Packet{}, fmt.Errorf("target name %q already exists", req.Name)
	}
	handler, err := b.findTargetKindLocked(req.Kind)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.New(
		req.Name,
		req.Kind,
		req.Options,
	); err != nil {
		return ipc.Packet{}, err
	}
	b.targetToKind[req.Name] = req.Kind
	return ipc.MakePacket(id, method.NewTargetResponse{}), nil
}

func (b *Base) handleNewFrontend(id uint32, req *method.NewFrontendRequest) (ipc.Packet, error) {
	listen, err := netip.ParseAddrPort(req.Listen)
	if err != nil {
		return ipc.Packet{}, fmt.Errorf("parse listen address %q: %w", req.Listen, err)
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	if _, exists := b.frontendToKind[req.Name]; exists {
		return ipc.Packet{}, fmt.Errorf("frontend name %q already exists", req.Name)
	}
	handler, err := b.findFrontendKindLocked(req.Kind)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.New(
		req.Name,
		req.Kind,
		req.Protocol,
		listen,
		req.Options,
	); err != nil {
		return ipc.Packet{}, err
	}
	b.frontendToKind[req.Name] = req.Kind
	return ipc.MakePacket(id, method.NewFrontendResponse{}), nil
}

func (b *Base) handleCloseTarget(id uint32, req *method.CloseTargetRequest) (ipc.Packet, error) {
	handler, err := b.findTargetHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}

	if err := handler.Close(req.Name); err != nil {
		return ipc.Packet{}, err
	}

	b.lock.Lock()
	delete(b.targetToKind, req.Name)
	b.lock.Unlock()
	return ipc.MakePacket(id, method.CloseTargetResponse{}), nil
}

func (b *Base) handleCloseFrontend(id uint32, req *method.CloseFrontendRequest) (ipc.Packet, error) {
	handler, err := b.findFrontendHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}

	if err := handler.Close(req.Name); err != nil {
		return ipc.Packet{}, err
	}

	b.lock.Lock()
	delete(b.frontendToKind, req.Name)
	b.lock.Unlock()
	return ipc.MakePacket(id, method.CloseFrontendResponse{}), nil
}

func (b *Base) handleWarmTarget(id uint32, req *method.WarmTargetRequest) (ipc.Packet, error) {
	handler, err := b.findTargetHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.Warm(req.Name); err != nil {
		return ipc.Packet{}, err
	}
	return ipc.MakePacket(id, method.WarmTargetResponse{}), nil
}

func (b *Base) handleDrainTarget(id uint32, req *method.DrainTargetRequest) (ipc.Packet, error) {
	handler, err := b.findTargetHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.Drain(req.Name); err != nil {
		return ipc.Packet{}, err
	}
	return ipc.MakePacket(id, method.DrainTargetResponse{}), nil
}

func (b *Base) handleStartFrontend(id uint32, req *method.StartFrontendRequest) (ipc.Packet, error) {
	handler, err := b.findFrontendHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.Start(req.Name); err != nil {
		return ipc.Packet{}, err
	}
	return ipc.MakePacket(id, method.StartFrontendResponse{}), nil
}

func (b *Base) handleStopFrontend(id uint32, req *method.StopFrontendRequest) (ipc.Packet, error) {
	handler, err := b.findFrontendHandler(req.Name)
	if err != nil {
		return ipc.Packet{}, err
	}
	if err := handler.Stop(req.Name); err != nil {
		return ipc.Packet{}, err
	}
	return ipc.MakePacket(id, method.StopFrontendResponse{}), nil
}

func (b *Base) findTargetKindLocked(kind string) (TargetHandler, error) {
	handler, exists := b.targetHandlers[kind]
	if !exists {
		return nil, fmt.Errorf("target kind %q is not registered", kind)
	}
	return handler, nil
}

func (b *Base) findFrontendKindLocked(kind string) (FrontendHandler, error) {
	handler, exists := b.frontendHandlers[kind]
	if !exists {
		return nil, fmt.Errorf("frontend kind %q is not registered", kind)
	}
	return handler, nil
}

func (b *Base) findTargetHandler(targetID string) (TargetHandler, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	kind, exists := b.targetToKind[targetID]
	if !exists {
		return nil, fmt.Errorf("target name %q not found", targetID)
	}
	return b.findTargetKindLocked(kind)
}

func (b *Base) findFrontendHandler(frontendID string) (FrontendHandler, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	kind, exists := b.frontendToKind[frontendID]
	if !exists {
		return nil, fmt.Errorf("frontend name %q not found", frontendID)
	}
	return b.findFrontendKindLocked(kind)
}

func (b *Base) shutdown(ctx context.Context, cause error) error {
	var err error
	b.lock.RLock()
	handlers := slices.Clone(b.shutdownHandlers)
	b.lock.RUnlock()
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		err = errors.Join(err, handler(ctx))
	}
	err = errors.Join(err, b.Close())
	return errors.Join(cause, err)
}
