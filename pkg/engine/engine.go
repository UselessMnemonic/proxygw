package engine

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
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

// Engine is the top-level runtime object for embedding or supervising a proxy
// gateway. Register driver kinds first, then create targets, then create and
// start frontends that point at those targets.
type Engine struct {
	dplane        *dataplane.Dataplane
	frontends     map[string]*frontend.Frontend
	frontendCtors map[string]frontend.HandlerCtor
	targets       map[string]*target.Target
	targetCtors   map[string]target.HandlerCtor
	logger        *slog.Logger

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
}

// New prepares a gateway engine with the given name. Keep this name stable
// across restarts for the same gateway instance because it is used for
// dataplane resources.
func New(ctx context.Context, name string) (*Engine, error) {
	ctx, cancel := context.WithCancel(ctx)
	logger := slog.Default().With("component", "engine", "name", name)

	plane, err := dataplane.New(ctx, name)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating dataplane: %w", err)
	}

	e := &Engine{
		dplane:        plane,
		frontends:     make(map[string]*frontend.Frontend),
		frontendCtors: make(map[string]frontend.HandlerCtor),
		targets:       make(map[string]*target.Target),
		targetCtors:   make(map[string]target.HandlerCtor),
		logger:        logger,

		lock:   sync.RWMutex{},
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,
		closed: false,
	}

	e.start()
	return e, nil
}

// Close requests shutdown. Close does not wait; call Wait when the caller must
// know that managed targets and frontends have finished.
func (e *Engine) Close() {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	e.cancel()
}

// Wait blocks until shutdown has been requested and all resource goroutines
// known to the engine have exited.
func (e *Engine) Wait() {
	<-e.ctx.Done()
	e.wg.Wait()
}

// Closed reports whether the engine is refusing new work because shutdown has
// started.
func (e *Engine) Closed() bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.closed
}

// AddFrontendKind makes a frontend implementation available to configuration
// and API calls. Names are conventionally namespace-qualified, such as
// "static:http".
func (e *Engine) AddFrontendKind(name string, kind frontend.HandlerCtor) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.frontendCtors[name]
	if exists {
		return ErrFrontendKindAlreadyRegistered
	}
	e.frontendCtors[name] = kind
	return nil
}

// FrontendKind returns the constructor registered with the given name, or nil
// when that kind is unknown.
func (e *Engine) FrontendKind(name string) frontend.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontendCtors[name]
}

// FrontendKinds returns a snapshot of known frontend constructors. The order is
// not stable.
func (e *Engine) FrontendKinds() []frontend.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.frontendCtors))
}

// DelFrontendKind unregisters a frontend implementation so new frontends can no
// longer use it.
func (e *Engine) DelFrontendKind(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.frontendCtors[name]
	if !exists {
		return ErrFrontendKindNotRegistered
	}
	for _, f := range e.frontends {
		if f.State() == frontend.Closed {
			continue
		}
	}
	delete(e.frontendCtors, name)
	return nil
}

// AddTargetKind makes a target implementation available to configuration and
// API calls. Names are conventionally namespace-qualified, such as "static:cmd".
func (e *Engine) AddTargetKind(name string, kind target.HandlerCtor) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.targetCtors[name]
	if exists {
		return ErrTargetKindAlreadyRegistered
	}
	e.targetCtors[name] = kind
	return nil
}

// GetTargetKind returns the constructor registered with the given name, or nil
// when that kind is unknown.
func (e *Engine) GetTargetKind(name string) target.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targetCtors[name]
}

// TargetKinds returns a snapshot of known target constructors. The order is not
// stable.
func (e *Engine) TargetKinds() []target.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.targetCtors))
}

// DelTargetKind unregisters a target implementation so new targets can no
// longer use it.
func (e *Engine) DelTargetKind(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.targetCtors[name]
	if !exists {
		return ErrTargetKindNotRegistered
	}
	for _, t := range e.targets {
		if t.State() == target.Closed {
			continue
		}
	}
	delete(e.targetCtors, name)
	return nil
}

// NewTarget creates a target from the given configuration. Target names must be
// unique for the lifetime of the engine unless a closed target is deleted first.
func (e *Engine) NewTarget(cfg config.Target) (*target.Target, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return nil, ErrClosed
	}

	_, exists := e.targets[cfg.Name]
	if exists {
		return nil, ErrTargetAlreadyRegistered
	}

	factory := e.targetCtors[cfg.Kind.String()]
	if factory == nil {
		return nil, fmt.Errorf("lookup kind %q: %w", cfg.Kind, ErrTargetKindNotRegistered)
	}

	driver, err := factory(cfg.Name, cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("driver for kind %q: %w", cfg.Kind, err)
	}

	dnat, err := e.dplane.NewDNATGroup(cfg.Name, cfg.IdleTimeout)
	if err != nil {
		return nil, fmt.Errorf("flow group: %w", err)
	}

	t, err := target.New(e.ctx, dnat, driver, cfg)
	if err != nil {
		return nil, errors.Join(
			err,
			dnat.Close(),
			driver.Close(),
		)
	}

	e.targets[t.Name()] = t
	e.joinTarget(t)
	return t, nil
}

// GetTarget returns the target with the given name, or nil when it is not
// registered.
func (e *Engine) GetTarget(name string) *target.Target {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targets[name]
}

// Targets returns a snapshot of registered targets. The order is not stable.
func (e *Engine) Targets() []*target.Target {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.targets))
}

// DelTarget forgets a closed target. Live targets must be closed by their owner
// before deletion.
func (e *Engine) DelTarget(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	t, exists := e.targets[name]
	if !exists {
		return nil
	}
	if t.State() != target.Closed {
		return ErrTargetInUse
	}
	delete(e.targets, name)
	return nil
}

// NewFrontend creates a frontend from the given configuration. The referenced
// target and endpoint must already exist.
func (e *Engine) NewFrontend(cfg config.Frontend) (*frontend.Frontend, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return nil, ErrClosed
	}

	_, exists := e.frontends[cfg.Name]
	if exists {
		return nil, ErrFrontendAlreadyRegistered
	}

	t := e.targets[cfg.Endpoint.Namespace]
	if t == nil {
		return nil, fmt.Errorf("lookup target %q: %w", cfg.Endpoint.Namespace, ErrTargetNotRegistered)
	}

	if t.State() == target.Closed {
		return nil, fmt.Errorf("lookup target %q: %w", cfg.Endpoint.Namespace, ErrTargetNotRegistered)
	}

	ctor := e.frontendCtors[cfg.Kind.String()]
	if ctor == nil {
		return nil, fmt.Errorf("frontend kind %q: %w", cfg.Kind, ErrFrontendKindNotRegistered)
	}

	driver, err := ctor(cfg.Name, cfg.Protocol, cfg.Listen, cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("driver for kind %q: %w", cfg.Kind, err)
	}

	endpoint, exists := t.Endpoint(cfg.Endpoint.Name)
	if !exists {
		return nil, fmt.Errorf("endpoint %q does not exist in target %q", cfg.Endpoint.Name, cfg.Endpoint.Namespace)
	}

	f, err := frontend.New(e.ctx, t, endpoint, driver, cfg)
	if err != nil {
		return nil, errors.Join(
			err,
			driver.Close(),
		)
	}

	e.frontends[f.Name()] = f
	e.joinFrontend(f)
	return f, nil
}

// GetFrontend returns the frontend with the given name, or nil when it is not
// registered.
func (e *Engine) GetFrontend(name string) *frontend.Frontend {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontends[name]
}

// Frontends returns a snapshot of registered frontends. The order is not
// stable.
func (e *Engine) Frontends() []*frontend.Frontend {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.frontends))
}

// DelFrontend forgets a closed frontend. Live frontends must be closed by their
// owner before deletion.
func (e *Engine) DelFrontend(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	f, exists := e.frontends[name]
	if !exists {
		return nil
	}
	if f.State() != frontend.Closed {
		return ErrFrontendInUse
	}
	delete(e.frontends, name)
	return nil
}

func (e *Engine) joinTarget(t *target.Target) {
	e.wg.Go(func() {
		t.Wait() // target guaranteed to be closed
		// TODO: Should we leave the dead weight or remove the target immediately ?
	})
}

func (e *Engine) joinFrontend(f *frontend.Frontend) {
	e.wg.Go(func() {
		f.Wait() // target guaranteed to be closed
		// TODO: Should we leave the dead weight or remove the frontend immediately ?
	})
}

func (e *Engine) start() {
	go func() {
		e.logger.Info("engine event loop started")
		<-e.ctx.Done()
		e.logger.Info("engine event loop stopping")
		e.Close()
		e.logger.Info("engine event loop stopped")
	}()
}
