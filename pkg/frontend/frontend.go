package frontend

import (
	"context"
	"errors"
	"fmt"
	"proxygw/pkg/config"
	"proxygw/pkg/dataplane"
	"proxygw/pkg/target"
	"sync"
)

type Frontend struct {
	name    string
	kind    string
	handler Handler

	target   *target.Target
	endpoint target.Endpoint
	requests chan State
	mapping  dataplane.DNATMapping

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	state  State
}

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

	mapping := dataplane.DNATMapping{
		Protocol:    cfg.Protocol,
		FlowTimeout: cfg.FlowTimeout,
		Source:      cfg.Listen,
		Destination: endpoint.Address,
	}
	err := target.DNATGroup().AddMappings(mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to bind target: %w", err)
	}
	err = target.DNATGroup().SetFlowTimeout(mapping.Source, mapping.Protocol, mapping.FlowTimeout)

	ctx, cancel := context.WithCancel(ctx)
	f := &Frontend{
		name:    cfg.Name,
		kind:    cfg.Kind.String(),
		handler: handler,

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

func (f *Frontend) Name() string {
	return f.name
}

func (f *Frontend) Kind() string {
	return f.kind
}

func (f *Frontend) Target() *target.Target {
	return f.target
}

func (f *Frontend) Endpoint() target.Endpoint {
	return f.endpoint
}

func (f *Frontend) Protocol() config.Protocol {
	return f.mapping.Protocol
}

func (f *Frontend) Listen() string {
	return f.mapping.Source.String()
}

func (f *Frontend) ProxyAddress() string {
	return f.mapping.Destination.String()
}

func (f *Frontend) State() State {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.state
}

func (f *Frontend) Error() error {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.err
}

func (f *Frontend) Wait() {
	<-f.ctx.Done()
	f.wg.Wait()
}

func (f *Frontend) Close() {
	f.cancel()
}

func (f *Frontend) Start() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	switch f.state {
	case Running, Starting:
		return true
	case Stopping:
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

func (f *Frontend) Stop() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	switch f.state {
	case Stopped, Stopping:
		return true
	case Starting:
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

func (f *Frontend) tryStart() {
	err := f.handler.Start()

	f.lock.Lock()
	defer f.lock.Unlock()
	f.err = err
	if f.err != nil {
		f.state = Stopping
		return
	}

	// the frontend is definitely running
	f.state = Running
}

func (f *Frontend) tryStop() {
	err := f.handler.Stop()

	f.lock.Lock()
	defer f.lock.Unlock()
	f.err = err
	if f.err != nil {
		f.state = Starting
		return
	}

	// the frontend is definitely stopped
	f.state = Stopped
}

func (f *Frontend) end() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.state = Closed
	f.err = errors.Join(
		f.target.DNATGroup().DelMappings(f.mapping),
		f.handler.Close(),
	)
}

func (f *Frontend) start() {
	f.wg.Go(func() {
		defer f.end()
		for {
			select {
			case <-f.ctx.Done():
				return
			case <-f.handler.ShouldWarm():
				f.target.Warm()
			case next := <-f.requests:
				switch next {
				case Starting:
					f.tryStart()
				case Stopping:
					f.tryStop()
				default:
					continue
				}
			}
		}
	})
}
