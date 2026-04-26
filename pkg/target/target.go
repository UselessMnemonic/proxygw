package target

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"proxygw/pkg/config"
	"proxygw/pkg/dataplane"
	"slices"
	"sync"
)

type Target struct {
	name    string
	kind    string
	handler Handler

	dnat      *dataplane.DNATGroup
	requests  chan State
	endpoints map[string]Endpoint

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	state  State
}

func New(ctx context.Context, dnat *dataplane.DNATGroup, handler Handler, cfg config.Target) (*Target, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if dnat == nil {
		return nil, errors.New("dnat is nil")
	}
	if handler == nil {
		return nil, errors.New("handler is nil")
	}

	endpoints := make(map[string]Endpoint, len(cfg.Endpoints))
	for _, epCfg := range cfg.Endpoints {
		next := Endpoint{
			Name:     epCfg.Name,
			Address:  epCfg.Address,
			Protocol: epCfg.Protocol,
		}
		if !next.IsValid() {
			return nil, fmt.Errorf("invalid endpoint: %v", next)
		}
		endpoints[next.Name] = next
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &Target{
		name:    cfg.Name,
		kind:    cfg.Kind,
		handler: handler,

		dnat:      dnat,
		requests:  make(chan State, 1),
		endpoints: endpoints,

		wg:     sync.WaitGroup{},
		lock:   sync.RWMutex{},
		ctx:    ctx,
		cancel: cancel,
		state:  Inactive,
		err:    nil,
	}
	t.start()
	return t, nil
}

func (t *Target) Name() string {
	return t.name
}

func (t *Target) Kind() string {
	return t.kind
}

func (t *Target) DNATGroup() *dataplane.DNATGroup {
	return t.dnat
}

func (t *Target) State() State {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.state
}

func (t *Target) Error() error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.err
}

func (t *Target) Endpoint(name string) (Endpoint, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	ep, exists := t.endpoints[name]
	return ep, exists
}

func (t *Target) Endpoints() []Endpoint {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return slices.Collect(maps.Values(t.endpoints))
}

func (t *Target) AddEndpoint(endpoint Endpoint) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.state == Closed {
		return ErrClosed
	}

	for _, ep := range t.endpoints {
		if endpoint.Overlaps(&ep) {
			return fmt.Errorf("endpoint %v overlaps with existing endpoint %v", endpoint, ep)
		}
	}

	t.endpoints[endpoint.Name] = endpoint
	return nil
}

func (t *Target) RemoveEndpoint(name string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.state == Closed {
		return ErrClosed
	}

	if _, exists := t.endpoints[name]; !exists {
		return nil
	}

	delete(t.endpoints, name)
	return nil
}

func (t *Target) Warm() bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	switch t.state {
	case Active, Warming:
		return true
	case Draining, Closed:
		return false
	case Inactive:
		t.err = nil
		t.state = Warming
		t.requests <- Warming
		return true
	default:
		panic(fmt.Sprintf("unknown target state: %d", t.state))
	}
}

func (t *Target) Drain() bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	switch t.state {
	case Inactive, Draining:
		return true
	case Warming, Closed:
		return false
	case Active:
		t.err = nil
		t.state = Draining
		t.requests <- Draining
		return true
	default:
		panic(fmt.Sprintf("unknown target state: %d", t.state))
	}
}

func (t *Target) Close() {
	t.cancel()
}

func (t *Target) Wait() {
	<-t.ctx.Done()
	t.wg.Wait()
}

func (t *Target) tryWarm() {
	err := t.handler.Warm()

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	if t.err != nil {
		t.state = Inactive
		return
	}

	// the target is definitely activated
	t.state = Active

	// try to enable DNAT
	err = t.dnat.Enable()
	if err != nil {
		t.err = fmt.Errorf("failed to enable DNAT: %w", err)
		return
	}
}

func (t *Target) tryDrain() {
	err := t.handler.Drain()

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	if t.err != nil {
		t.state = Active
		return
	}

	// the target is definitely inactive
	t.state = Inactive

	// try to disable DNAT
	err = t.dnat.Disable()
	if err != nil {
		t.err = fmt.Errorf("failed to disable DNAT: %w", err)
		return
	}
}

func (t *Target) end() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.state = Closed
	t.err = errors.Join(
		t.dnat.Close(),
		t.handler.Close(),
	)
}

func (t *Target) start() {
	t.wg.Go(func() {
		defer t.end()
		for {
			select {
			case <-t.ctx.Done():
				return
			case next := <-t.requests:
				switch next {
				case Warming:
					t.tryWarm()
				case Draining:
					t.tryDrain()
				default:
					continue
				}
			}
		}
	})
}
