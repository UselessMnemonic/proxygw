package dataplane

import (
	"context"
	"sync"

	"github.com/google/nftables"
	"github.com/ti-mo/conntrack"
)

type Dataplane struct {
	ct         *conntrack.Conn
	nft        *nftables.Conn
	dnatGroups map[string]*DNATGroup

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

func New(ctx context.Context) (*Dataplane, error) {
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
		ct:         ct,
		nft:        nft,
		dnatGroups: make(map[string]*DNATGroup),

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

func (d *Dataplane) Error() error {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.err
}

func (d *Dataplane) Close() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.closed = true
	d.cancel()
}

func (d *Dataplane) Wait() {
	<-d.ctx.Done()
	d.wg.Wait()
}
