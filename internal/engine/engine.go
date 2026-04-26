package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"proxygw/internal/dataplane"
	frontend2 "proxygw/internal/frontend"
	target2 "proxygw/internal/target"
	"proxygw/pkg/config"
	"slices"
	"sync"
)

type Engine struct {
	dplane        *dataplane.Dataplane
	frontends     map[string]*frontend2.Frontend
	frontendCtors map[string]frontend2.HandlerCtor
	targets       map[string]*target2.Target
	targetCtors   map[string]target2.HandlerCtor

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
		frontends:     make(map[string]*frontend2.Frontend),
		frontendCtors: make(map[string]frontend2.HandlerCtor),
		targets:       make(map[string]*target2.Target),
		targetCtors:   make(map[string]target2.HandlerCtor),

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

func (e *Engine) Closed() bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.closed
}

func (e *Engine) AddFrontendKind(name string, kind frontend2.HandlerCtor) error {
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

func (e *Engine) FrontendKind(name string) frontend2.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontendCtors[name]
}

func (e *Engine) FrontendKinds() []frontend2.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.frontendCtors))
}

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
		if f.State() == frontend2.Closed {
			continue
		}
	}
	delete(e.frontendCtors, name)
	return nil
}

func (e *Engine) AddTargetKind(name string, kind target2.HandlerCtor) error {
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

func (e *Engine) GetTargetKind(name string) target2.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targetCtors[name]
}

func (e *Engine) TargetKinds() []target2.HandlerCtor {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.targetCtors))
}

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
		if t.State() == target2.Closed {
			continue
		}
	}
	delete(e.targetCtors, name)
	return nil
}

func (e *Engine) NewTarget(cfg config.Target) (*target2.Target, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return nil, ErrClosed
	}

	_, exists := e.targets[cfg.Name]
	if exists {
		return nil, ErrTargetAlreadyRegistered
	}

	factory := e.targetCtors[cfg.Kind]
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

	t, err := target2.New(e.ctx, dnat, driver, cfg)
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

func (e *Engine) GetTarget(name string) *target2.Target {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.targets[name]
}

func (e *Engine) Targets() []*target2.Target {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.targets))
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
	if t.State() != target2.Closed {
		return ErrTargetInUse
	}
	delete(e.targets, name)
	return nil
}

func (e *Engine) NewFrontend(cfg config.Frontend) (*frontend2.Frontend, error) {
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

	if t.State() == target2.Closed {
		return nil, fmt.Errorf("lookup target %q: %w", cfg.Endpoint.TargetName, ErrTargetNotRegistered)
	}

	ctor := e.frontendCtors[cfg.Kind]
	if ctor == nil {
		return nil, fmt.Errorf("frontend kind %q: %w", cfg.Kind, ErrFrontendKindNotRegistered)
	}

	driver, err := ctor(cfg.Name, cfg.Protocol, cfg.Listen, cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("driver for kind %q: %w", cfg.Kind, err)
	}

	endpoint, exists := t.Endpoint(cfg.Endpoint.EndpointName)
	if !exists {
		return nil, fmt.Errorf("endpoint %q does not exist in target %q", cfg.Endpoint.EndpointName, cfg.Endpoint.TargetName)
	}

	f, err := frontend2.New(e.ctx, t, endpoint, driver, cfg)
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

func (e *Engine) GetFrontend(name string) *frontend2.Frontend {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.frontends[name]
}

func (e *Engine) Frontends() []*frontend2.Frontend {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return slices.Collect(maps.Values(e.frontends))
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
	if f.State() != frontend2.Closed {
		return ErrFrontendInUse
	}
	delete(e.frontends, name)
	return nil
}

func (e *Engine) joinTarget(t *target2.Target) {
	e.wg.Go(func() {
		t.Wait() // target guaranteed to be closed
		// TODO: Should we leave the dead weight or remove the target immediately ?
	})
}

func (e *Engine) joinFrontend(f *frontend2.Frontend) {
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
