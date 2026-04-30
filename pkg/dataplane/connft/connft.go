package connft

import (
	"errors"
	"log/slog"
	"maps"
	"sync"

	"github.com/google/nftables"
	"github.com/ti-mo/conntrack"
)

// Connft is an implementation of Dataplane which leverages nftables and conntrack.
type Connft struct {
	name   string
	ct     *conntrack.Conn
	nft    *nftables.Conn
	groups map[string]*ConnftGroup
	logger *slog.Logger

	table            *nftables.Table
	dnatSpecific4    *nftables.Set
	dnatSpecific6    *nftables.Set
	dnatWildcard4    *nftables.Set
	dnatWildcard6    *nftables.Set
	prerouteNATChain *nftables.Chain
	outputNATChain   *nftables.Chain
	inputFilterChain *nftables.Chain

	lock   sync.RWMutex
	closed bool
}

// New creates a new Dataplane backed by a flow table with the given name
func New(name string) (*Connft, error) {
	ct, err := conntrack.Dial(nil)
	if err != nil {
		return nil, err
	}

	nft, err := nftables.New()
	if err != nil {
		_ = ct.Close()
		return nil, err
	}

	d := &Connft{
		name:   name,
		ct:     ct,
		nft:    nft,
		groups: make(map[string]*ConnftGroup),
		logger: slog.Default().With("component", "dataplane", "table", name),

		lock:   sync.RWMutex{},
		closed: false,
	}

	err = d.ensureTableAdded()
	if err != nil {
		d.Close()
		return nil, err
	}

	return d, nil
}

// Name returns the name
func (d *Connft) Name() string {
	return d.name
}

// Close tears down dataplane resources. It is safe to call more than once.
func (d *Connft) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true

	groups := maps.Clone(d.groups)
	var err error

	for _, g := range groups {
		err = errors.Join(err, g.close())
	}
	err = errors.Join(err, d.ensureTableDeleted())
	d.ct.Close()
	return err
}
