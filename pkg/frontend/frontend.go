package frontend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

// Frontend is a managed listener that can be started and stopped independently
// while keeping its configured route to a target endpoint.
type Frontend struct {
	name    string
	kind    string
	handler Handler
	logger  *slog.Logger

	target   *target.Target
	endpoint target.Endpoint
	requests chan State
	mapping  dataplane.Mapping

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	state  State
}

// New builds a frontend around a handler and reserves its dataplane
// mapping. The caller still needs to call Start before the listener accepts
// traffic. Dataplane binding errors are wrapped.
func New(ctx context.Context, target *target.Target, endpoint target.Endpoint, handler Handler, cfg config.Frontend) (*Frontend, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if target == nil {
		return nil, errors.New("target is nil")
	}
	if handler == nil {
		return nil, errors.New("handler is nil")
	}

	mapping := dataplane.Mapping{
		Protocol:    cfg.Protocol,
		Source:      cfg.Listen,
		Destination: endpoint.Address,
		Timeout:     cfg.FlowTimeout,
	}
	err := target.Group().AddMappings(mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to bind target: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	f := &Frontend{
		name:    cfg.Name,
		kind:    cfg.Kind.String(),
		handler: handler,
		logger:  slog.Default().With("component", "frontend", "name", cfg.Name, "kind", cfg.Kind.String(), "target", target.Name(), "endpoint", endpoint.Name),

		target:   target,
		endpoint: endpoint,
		requests: make(chan State, 1),
		mapping:  mapping,

		wg:     sync.WaitGroup{},
		lock:   sync.RWMutex{},
		ctx:    ctx,
		cancel: cancel,
		state:  Stopped,
		err:    err,
	}
	f.start()
	return f, nil
}

// Name returns the configuration name for this frontend.
func (f *Frontend) Name() string {
	return f.name
}

// Kind returns the frontend implementation name used to create this instance.
func (f *Frontend) Kind() string {
	return f.kind
}

// Target returns the target this frontend forwards to.
func (f *Frontend) Target() *target.Target {
	return f.target
}

// Endpoint returns the target endpoint selected by this frontend.
func (f *Frontend) Endpoint() target.Endpoint {
	return f.endpoint
}

// Protocol returns the transport protocol accepted by this frontend.
func (f *Frontend) Protocol() config.Protocol {
	return f.mapping.Protocol
}

// Listen returns the local address clients connect to.
func (f *Frontend) Listen() string {
	return f.mapping.Source.String()
}

// ProxyAddress returns the current backend address used by the dataplane.
func (f *Frontend) ProxyAddress() string {
	return f.mapping.Destination.String()
}

// State returns the frontend's latest lifecycle state.
func (f *Frontend) State() State {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.state
}

// Error returns the last lifecycle error, if any.
func (f *Frontend) Error() error {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.err
}

// Wait blocks until the frontend has closed and its event loop has exited.
func (f *Frontend) Wait() {
	<-f.ctx.Done()
	f.wg.Wait()
}

// Close requests permanent shutdown of the frontend. It does not wait for the
// handler to finish closing; call Wait to observe shutdown completion.
func (f *Frontend) Close() {
	f.lock.Lock()
	if f.state == Closed {
		f.lock.Unlock()
		return
	}
	f.state = Closed
	f.lock.Unlock()
	f.cancel()
}

// Start requests that the frontend begin accepting traffic. It returns false
// when a conflicting stop is already in progress.
func (f *Frontend) Start() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	switch f.state {
	case Running, Starting:
		return true
	case Stopping, Closed:
		return false
	case Stopped:
		f.err = nil
		f.state = Starting
		f.requests <- Starting
		return true
	default:
		panic(fmt.Sprintf("unknown frontend state: %d", f.state))
	}
}

// Stop requests that the frontend stop accepting traffic. It returns false when
// a conflicting start is already in progress.
func (f *Frontend) Stop() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	switch f.state {
	case Stopped, Stopping:
		return true
	case Starting, Closed:
		return false
	case Running:
		f.err = nil
		f.state = Stopping
		f.requests <- Stopping
		return true
	default:
		panic(fmt.Sprintf("unknown frontend state: %d", f.state))
	}
}

func (f *Frontend) startBlocking() {
	f.lock.RLock()
	if f.state == Closed {
		f.lock.RUnlock()
		return
	}
	f.lock.RUnlock()

	f.logger.Info("start started", "listen", f.Listen())
	err := f.handler.Start()

	f.lock.Lock()
	defer f.lock.Unlock()
	if f.state == Closed {
		return
	}
	f.err = err
	if f.err != nil {
		f.state = Stopping
		f.logger.Error("start completed", "err", f.err)
		return
	}

	// the frontend is definitely running
	f.state = Running
	f.logger.Info("start completed", "listen", f.Listen())
}

func (f *Frontend) stopBlocking() {
	f.lock.RLock()
	if f.state == Closed {
		f.lock.RUnlock()
		return
	}
	f.lock.RUnlock()

	f.logger.Info("stop started", "listen", f.Listen())
	err := f.handler.Stop()

	f.lock.Lock()
	defer f.lock.Unlock()
	if f.state == Closed {
		return
	}
	f.err = err
	if f.err != nil {
		f.state = Starting
		f.logger.Error("stop completed", "err", f.err)
		return
	}

	// the frontend is definitely stopped
	f.state = Stopped
	f.logger.Info("stop completed", "listen", f.Listen())
}

func (f *Frontend) endBlocking() {
	f.lock.Lock()
	f.state = Closed
	f.lock.Unlock()

	err := errors.Join(
		f.handler.Close(),
		f.target.Group().DelMappings(f.mapping),
	)

	f.lock.Lock()
	defer f.lock.Unlock()
	f.err = err
	if f.err != nil {
		f.logger.Error("close completed", "err", f.err)
		return
	}
	f.logger.Info("close completed")
}

func (f *Frontend) start() {
	f.wg.Go(func() {
		f.logger.Info("event loop started")
		defer func() {
			f.endBlocking()
			if err := f.Error(); err != nil {
				f.logger.Error("event loop stopped", "state", f.State().String(), "err", err)
				return
			}
			f.logger.Info("event loop stopped", "state", f.State().String())
		}()
		for {
			select {
			case <-f.ctx.Done():
				return
			case <-f.handler.ShouldWarm():
				ok := f.target.Warm()
				f.logger.Info("warm signal received", "warm", ok)
			case next := <-f.requests:
				switch next {
				case Starting:
					f.logger.Info("start requested")
					f.startBlocking()
				case Stopping:
					f.logger.Info("stop requested")
					f.stopBlocking()
				default:
					continue
				}
			}
		}
	})
}
