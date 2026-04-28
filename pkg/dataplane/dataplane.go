package dataplane

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/nftables"
	"github.com/ti-mo/conntrack"
)

// Dataplane owns the host networking resources used to route frontend traffic
// to warmed targets.
type Dataplane struct {
	name       string
	ct         *conntrack.Conn
	nft        *nftables.Conn
	dnatGroups map[string]*DNATGroup
	logger     *slog.Logger

	table            *nftables.Table
	dnatSpecific4    *nftables.Set
	dnatSpecific6    *nftables.Set
	dnatWildcard4    *nftables.Set
	dnatWildcard6    *nftables.Set
	prerouteNATChain *nftables.Chain
	inputFilterChain *nftables.Chain

	wg     sync.WaitGroup
	lock   sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	closed bool
}

// New prepares dataplane resources for the given gateway name. Call Close when
// the owning engine or process is shutting down.
func New(ctx context.Context, name string) (*Dataplane, error) {
	ct, err := conntrack.Dial(nil)
	if err != nil {
		return nil, err
	}

	nft, err := nftables.New()
	if err != nil {
		_ = ct.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	d := &Dataplane{
		name:       name,
		ct:         ct,
		nft:        nft,
		dnatGroups: make(map[string]*DNATGroup),
		logger:     slog.Default().With("component", "dataplane", "table", name),

		ctx:    ctx,
		cancel: cancel,
		err:    nil,
		lock:   sync.RWMutex{},
		closed: false,
	}

	err = d.ensureTableAdded()
	if err != nil {
		d.Close()
		return nil, err
	}

	d.start()
	return d, nil
}

// Error returns the last dataplane error observed by the background worker, if
// any.
func (d *Dataplane) Error() error {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.err
}

// Close requests dataplane shutdown. It is safe to call more than once.
func (d *Dataplane) Close() {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	d.cancel()
}

// Wait blocks until dataplane shutdown has completed.
func (d *Dataplane) Wait() {
	<-d.ctx.Done()
	d.wg.Wait()
}
