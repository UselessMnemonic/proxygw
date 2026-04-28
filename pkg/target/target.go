package target

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"
)

// Target is a managed backend that can be warmed before traffic is routed to it
// and drained after it becomes idle.
type Target struct {
	name    string
	kind    string
	handler Handler
	logger  *slog.Logger

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

// New builds a target around a driver handler. The returned target starts
// inactive; call Warm before expecting it to serve traffic.
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
		kind:    cfg.Kind.String(),
		handler: handler,
		logger:  slog.Default().With("component", "target", "name", cfg.Name, "kind", cfg.Kind.String()),

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

// Name returns the configuration name for this target.
func (t *Target) Name() string {
	return t.name
}

// Kind returns the target implementation name used to create this instance.
func (t *Target) Kind() string {
	return t.kind
}

// DNATGroup returns the dataplane group owned by this target.
func (t *Target) DNATGroup() *dataplane.DNATGroup {
	return t.dnat
}

// State returns the target's latest lifecycle state.
func (t *Target) State() State {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.state
}

// Error returns the last lifecycle error, if any.
func (t *Target) Error() error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.err
}

// Endpoint returns the configured endpoint with the given name.
func (t *Target) Endpoint(name string) (Endpoint, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	ep, exists := t.endpoints[name]
	return ep, exists
}

// Endpoints returns a snapshot of this target's configured endpoints. The order
// is not stable.
func (t *Target) Endpoints() []Endpoint {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return slices.Collect(maps.Values(t.endpoints))
}

// AddEndpoint adds an endpoint for future frontend bindings.
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

// RemoveEndpoint removes an endpoint by name. Existing frontend bindings are
// not automatically rewritten.
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

// Warm requests that the target prepare to receive traffic. It returns false
// when a conflicting drain or close is in progress.
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

// Drain requests that the target stop serving traffic and release resources. It
// returns false when a conflicting warm or close is in progress.
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

// Close requests permanent shutdown of the target.
func (t *Target) Close() {
	t.cancel()
}

// Wait blocks until the target has closed and its event loop has exited.
func (t *Target) Wait() {
	<-t.ctx.Done()
	t.wg.Wait()
}

func (t *Target) tryWarm() {
	t.logger.Info("warm started")
	err := t.handler.Warm()

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	if t.err != nil {
		t.state = Inactive
		t.logger.Error("warm failed", "err", t.err)
		return
	}

	// the target is definitely activated
	t.state = Active

	// try to enable DNAT
	err = t.dnat.Enable()
	if err != nil {
		t.err = fmt.Errorf("failed to enable DNAT: %w", err)
		t.logger.Error("warm failed", "err", t.err)
		return
	}
	t.logger.Info("warm completed")
}

func (t *Target) tryDrain() {
	t.logger.Info("drain started")
	err := t.handler.Drain()

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	if t.err != nil {
		t.state = Active
		t.logger.Error("drain failed", "err", t.err)
		return
	}

	// the target is definitely inactive
	t.state = Inactive

	// try to disable DNAT
	err = t.dnat.Disable()
	if err != nil {
		t.err = fmt.Errorf("failed to disable DNAT: %w", err)
		t.logger.Error("drain failed", "err", t.err)
		return
	}
	t.logger.Info("drain completed")
}

func (t *Target) end() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.state = Closed
	t.err = errors.Join(
		t.dnat.Close(),
		t.handler.Close(),
	)
	if t.err != nil {
		t.logger.Error("close failed", "err", t.err)
		return
	}
	t.logger.Info("close completed")
}

func (t *Target) start() {
	t.wg.Go(func() {
		t.logger.Info("event loop started")
		defer func() {
			t.end()
			if err := t.Error(); err != nil {
				t.logger.Error("event loop stopped", "state", t.State().String(), "err", err)
				return
			}
			t.logger.Info("event loop stopped", "state", t.State().String())
		}()
		for {
			select {
			case <-t.ctx.Done():
				return
			case next := <-t.requests:
				switch next {
				case Warming:
					t.logger.Info("warm requested")
					t.tryWarm()
				case Draining:
					t.logger.Info("drain requested")
					t.tryDrain()
				default:
					continue
				}
			case timeout := <-t.dnat.Timeout():
				t.logger.Info("timeout event", "timestamp", timeout.Timestamp)
				go func() {
					t.Drain()
				}()
			}
		}
	})
}
