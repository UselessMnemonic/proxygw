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
	timeout config.TTL
	handler Handler
	logger  *slog.Logger

	group     dataplane.Group
	requests  chan State
	endpoints map[string]Endpoint

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	state  State
}

// New builds a target around a handler. The returned target starts
// inactive; call Warm before expecting it to serve traffic.
func New(ctx context.Context, group dataplane.Group, handler Handler, cfg config.Target) (*Target, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if group == nil {
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
		timeout: cfg.IdleTimeout,
		handler: handler,
		logger:  slog.Default().With("component", "target", "name", cfg.Name),

		group:     group,
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

// Timeout returns the time after which a stale target is Drained by the engine
func (t *Target) Timeout() config.TTL {
	return t.timeout
}

// SetTimeout updates this Target's timeout
func (t *Target) SetTimeout(timeout config.TTL) {
	t.timeout = timeout
}

// Kind returns the target implementation name used to create this instance.
func (t *Target) Kind() string {
	return t.kind
}

// Group returns the dataplane group owned by this target.
func (t *Target) Group() dataplane.Group {
	return t.group
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

// AddEndpoint adds an endpoint for future frontend bindings. Returns ErrClosed.
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
// not automatically rewritten. Returns ErrClosed.
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
		t.logger.Debug("submitting Warming")
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
		t.logger.Debug("submitting Draining")
		t.requests <- Draining
		return true
	default:
		panic(fmt.Sprintf("unknown target state: %d", t.state))
	}
}

// Close requests permanent shutdown of the target. It does not wait for the
// handler to finish closing; call Wait to observe shutdown completion.
func (t *Target) Close() {
	t.lock.Lock()
	if t.state == Closed {
		t.lock.Unlock()
		return
	}
	t.state = Closed
	t.lock.Unlock()
	t.logger.Debug("submitting cancel")
	t.cancel()
}

// Wait blocks until the target has closed and its event loop has exited.
func (t *Target) Wait() {
	<-t.ctx.Done()
	t.wg.Wait()
}

func (t *Target) warmBlocking() {
	t.logger.Info("warm requested")
	t.lock.RLock()
	if t.state == Closed {
		t.lock.RUnlock()
		t.logger.Info("warm skipped because target is closed")
		return
	}
	t.lock.RUnlock()

	t.logger.Debug("calling into handler warm")
	err := t.handler.Warm()
	t.logger.Debug("exiting handler warm")

	t.lock.Lock()
	defer t.lock.Unlock()
	if t.state == Closed {
		t.logger.Info("warm ended early because target is closed")
		return
	}
	t.err = err
	if t.err != nil {
		t.state = Inactive
		t.logger.Error("warm failed", "err", t.err)
		return
	}

	// the target is definitely activated
	t.state = Active

	// try to enable DNAT
	err = t.group.Enable()
	if err != nil {
		t.err = fmt.Errorf("failed to enable DNAT: %w", err)
		t.logger.Error("warm completed", "err", t.err)
		return
	}
	t.logger.Info("warm completed")
}

func (t *Target) drainBlocking() {
	t.logger.Info("drain requested")
	t.lock.RLock()
	if t.state == Closed {
		t.lock.RUnlock()
		t.logger.Info("drain skipped because target is closed")
		return
	}
	t.lock.RUnlock()

	t.logger.Debug("calling into handler drain")
	err := t.handler.Drain()
	t.logger.Debug("exiting handler drain")

	t.lock.Lock()
	defer t.lock.Unlock()
	if t.state == Closed {
		t.logger.Info("drain ended early because target is closed")
		return
	}
	t.err = err
	if t.err != nil {
		t.state = Active
		t.logger.Error("drain failed", "err", t.err)
		return
	}

	// the target is definitely inactive
	t.state = Inactive

	// try to disable DNAT
	err = t.group.Disable()
	if err != nil {
		t.err = fmt.Errorf("failed to disable DNAT: %w", err)
		t.logger.Error("drain completed", "err", t.err)
		return
	}
	t.logger.Info("drain completed")
}

func (t *Target) endBlocking() {
	t.logger.Info("close requested")
	t.lock.Lock()
	t.state = Closed
	t.lock.Unlock()

	t.logger.Debug("calling into handler close")
	err := t.handler.Close()
	t.logger.Debug("exiting handler close")

	err = errors.Join(err, t.group.Close())

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	if t.err != nil {
		t.logger.Error("close completed", "err", t.err)
		return
	}
	t.logger.Info("close completed")
}

func (t *Target) start() {
	t.wg.Go(func() {
		defer t.logger.Debug("event loop ended")
		t.logger.Debug("event loop started")
		defer t.endBlocking()
		for {
			select {
			case <-t.ctx.Done():
				t.logger.Debug("close signal received")
				return
			case next := <-t.requests:
				switch next {
				case Warming:
					t.warmBlocking()
				case Draining:
					t.drainBlocking()
				default:
					t.logger.Error("unknown signal received", "signal", next)
					continue
				}
			}
		}
	})
}
