package engine

import (
	"context"
	"fmt"
	"proxygw/internal/dataplane"
	"proxygw/internal/frontend"
	"proxygw/internal/target"
	"proxygw/pkg/config"
	"sync"
)

type Engine struct {
	dplane        *dataplane.Dataplane
	frontends     map[string]*frontend.Frontend
	frontendKinds map[string]frontend.Kind
	targets       map[string]*target.Target
	targetKinds   map[string]target.Kind

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
}

func New(ctx context.Context) (*Engine, error) {
	ctx, cancel := context.WithCancel(ctx)

	plane, err := dataplane.New(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating dataplane: %w", err)
	}

	e := &Engine{
		dplane:        plane,
		frontends:     make(map[string]*frontend.Frontend),
		frontendKinds: make(map[string]frontend.Kind),
		targets:       make(map[string]*target.Target),
		targetKinds:   make(map[string]target.Kind),

		lock:   sync.RWMutex{},
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,
		closed: false,
	}
	e.start()
	return e, nil
}

func (e *Engine) Close() {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.closed = true
	e.cancel()
}

func (e *Engine) Wait() {
	<-e.ctx.Done()
	e.wg.Wait()
}

func (e *Engine) AddFrontendKind(kind frontend.Kind) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.frontendKinds[kind.Name()]
	if exists {
		return ErrFrontendKindAlreadyRegistered
	}
	e.frontendKinds[kind.Name()] = kind
	return nil
}

func (e *Engine) GetFrontendKind(name string) frontend.Kind {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontendKinds[name]
}

func (e *Engine) DelFrontendKind(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.frontendKinds[name]
	if !exists {
		return ErrFrontendKindNotRegistered
	}
	for _, f := range e.frontends {
		if f.State() == frontend.Closed {
			continue
		}
		if f.Kind().Name() == name {
			return ErrFrontendKindInUse
		}
	}
	delete(e.frontendKinds, name)
	return nil
}

func (e *Engine) AddTargetKind(kind target.Kind) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.targetKinds[kind.Name()]
	if exists {
		return ErrTargetKindAlreadyRegistered
	}
	e.targetKinds[kind.Name()] = kind
	return nil
}

func (e *Engine) GetTargetKind(name string) target.Kind {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targetKinds[name]
}

func (e *Engine) DelTargetKind(name string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return ErrClosed
	}

	_, exists := e.targetKinds[name]
	if !exists {
		return ErrTargetKindNotRegistered
	}
	for _, t := range e.targets {
		if t.State() == target.Closed {
			continue
		}
		if t.Kind().Name() == name {
			return ErrTargetKindInUse
		}
	}
	delete(e.targetKinds, name)
	return nil
}

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

	kind := e.targetKinds[cfg.Kind]
	if kind == nil {
		return nil, fmt.Errorf("lookup kind %q: %w", cfg.Kind, ErrTargetKindNotRegistered)
	}

	dnatGroup, err := e.dplane.NewDNATGroup(cfg.Name, cfg.IdleTimeout)
	if err != nil {
		return nil, fmt.Errorf("flow group: %w", err)
	}

	driver, err := kind.New(cfg.Options)
	if err != nil {
		dnatGroup.Close()
		return nil, fmt.Errorf("driver for kind %q: %w", cfg.Kind, err)
	}

	t, err := target.New(e.ctx, dnatGroup, driver, cfg)
	if err != nil {
		dnatGroup.Close()
		driver.Close()
		return nil, err
	}

	e.targets[t.Name()] = t
	e.joinTarget(t)
	return t, nil
}

func (e *Engine) GetTarget(name string) *target.Target {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targets[name]
}

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

	t := e.targets[cfg.Endpoint.TargetName]
	if t == nil {
		return nil, fmt.Errorf("lookup target %q: %w", cfg.Endpoint.TargetName, ErrTargetNotRegistered)
	}

	if t.State() == target.Closed {
		return nil, fmt.Errorf("lookup target %q: %w", cfg.Endpoint.TargetName, ErrTargetNotRegistered)
	}

	kind := e.frontendKinds[cfg.Kind]
	if kind == nil {
		return nil, fmt.Errorf("frontend kind %q: %w", cfg.Kind, ErrFrontendKindNotRegistered)
	}

	driver, err := kind.New(cfg.Protocol, cfg.Listen, cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("driver for kind %q: %w", cfg.Kind, err)
	}

	endpoint, exists := t.Endpoint(cfg.Endpoint.EndpointName)
	if !exists {
		return nil, fmt.Errorf("endpoint %q does not exist in target %q", cfg.Endpoint.EndpointName, cfg.Endpoint.TargetName)
	}

	f, err := frontend.New(e.ctx, t, endpoint, driver, cfg)
	if err != nil {
		driver.Close()
		return nil, err
	}

	e.frontends[f.Name()] = f
	e.joinFrontend(f)
	return f, nil
}

func (e *Engine) GetFrontend(name string) *frontend.Frontend {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontends[name]
}

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
		<-e.ctx.Done()
		e.Close()
	}()
}
