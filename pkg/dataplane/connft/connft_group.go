package connft

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"

	"github.com/google/nftables"
)

type dnatKey struct {
	addr  netip.AddrPort
	proto config.Protocol
}

type flowInfo struct {
	timeoutObj  *nftables.NamedObj
	timeoutRule *nftables.Rule
}

// ConnftGroup collects the mappings for one target. Enable and Disable switch the
// whole group in and out of the live dataplane.
type ConnftGroup struct {
	dplane        *Connft
	name          string
	mappingsBySrc map[dnatKey]dataplane.Mapping
	mappingsByDst map[dnatKey]dataplane.Mapping
	flowInfoBySrc map[dnatKey]flowInfo
	lastSeen      time.Time
	ttl           config.TTL
	enabled       bool
	closed        bool
}

// NewGroup registers a new valid Group to the underlying dataplane. Returns
// dataplane.ErrClosed or dataplane.ErrGroupAlreadyRegistered.
func (d *Connft) NewGroup(name string) (dataplane.Group, error) {
	return d.NewConnftGroup(name)
}

// NewConnftGroup reserves a mapping group with the given name. Returns
// dataplane.ErrClosed or dataplane.ErrGroupAlreadyRegistered.
func (d *Connft) NewConnftGroup(name string) (*ConnftGroup, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return nil, dataplane.ErrClosed
	}
	if _, exists := d.groups[name]; exists {
		return nil, dataplane.ErrGroupAlreadyRegistered
	}

	dg := &ConnftGroup{
		dplane:        d,
		name:          name,
		mappingsBySrc: make(map[dnatKey]dataplane.Mapping),
		mappingsByDst: make(map[dnatKey]dataplane.Mapping),
		flowInfoBySrc: make(map[dnatKey]flowInfo),
		lastSeen:      time.Now(),
		enabled:       false,
		closed:        false,
	}
	d.groups[name] = dg
	return dg, nil
}

// Name returns the group name.
func (dg *ConnftGroup) Name() string {
	return dg.name
}

// IsEnabled reports whether this group's mappings are currently installed.
func (dg *ConnftGroup) IsEnabled() bool {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	if dg.closed || dg.dplane.closed {
		return false
	}
	return dg.enabled
}

// Timeout returns the conntrack timeout configured for a mapping. Returns
// dataplane.ErrClosed or dataplane.ErrNoSuchMapping.
func (dg *ConnftGroup) Timeout(protocol config.Protocol, source netip.AddrPort) (config.TTL, error) {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()

	if dg.closed || dg.dplane.closed {
		return 0, dataplane.ErrClosed
	}

	m, exists := dg.mappingsBySrc[dnatKey{source, protocol}]
	if !exists {
		return 0, dataplane.ErrNoSuchMapping
	}
	return m.Timeout, nil
}

// SetTimeout changes the conntrack timeout for an existing mapping. Returns
// dataplane.ErrClosed or dataplane.ErrNoSuchMapping.
func (dg *ConnftGroup) SetTimeout(protocol config.Protocol, source netip.AddrPort, timeout config.TTL) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	m, exists := dg.mappingsBySrc[dnatKey{source, protocol}]
	if !exists {
		return dataplane.ErrNoSuchMapping
	}
	if m.Timeout == timeout {
		return nil
	}
	if err := dg.ensureTimeoutSet(protocol, source, timeout); err != nil {
		return err
	}
	m.Timeout = timeout
	dg.mappingsBySrc[dnatKey{source, protocol}] = m
	dg.mappingsByDst[dnatKey{m.Destination, m.Protocol}] = m
	return nil
}

// Mappings returns a snapshot of this group's mappings. The order is not
// stable.
func (dg *ConnftGroup) Mappings() []dataplane.Mapping {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	if dg.closed || dg.dplane.closed {
		return nil
	}
	return dg.mappings()
}

// AddMappings adds one or more non-overlapping mappings to the group. Returns
// dataplane.ErrClosed.
func (dg *ConnftGroup) AddMappings(mappings ...dataplane.Mapping) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	return dg.addMappings(mappings)
}

// DelMappings removes exact mappings from the group. Returns
// dataplane.ErrClosed or dataplane.ErrNoSuchMapping.
func (dg *ConnftGroup) DelMappings(mappings ...dataplane.Mapping) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	return dg.delMappings(mappings)
}

// ClearMappings removes every mapping from the group. Returns
// dataplane.ErrClosed.
func (dg *ConnftGroup) ClearMappings() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	return dg.clearMappings()
}

// Enable installs this group's mappings so traffic can be forwarded. Returns
// dataplane.ErrClosed.
func (dg *ConnftGroup) Enable() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	if dg.enabled {
		return nil
	}

	mappings := dg.mappings()
	err := dg.ensureDNATAdded(mappings)
	if err == nil {
		dg.enabled = true
	}
	return err
}

// Disable removes this group's mappings from the live dataplane. Returns
// dataplane.ErrClosed.
func (dg *ConnftGroup) Disable() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()

	if dg.closed || dg.dplane.closed {
		return dataplane.ErrClosed
	}

	if !dg.enabled {
		return nil
	}

	mappings := dg.mappings()
	err := dg.ensureDNATDeleted(mappings)
	if err == nil {
		dg.enabled = false
	}
	return err
}

// Close evicts this group from its parent Dataplane and renders this group
// unusable. It is safe to call more than once.
func (dg *ConnftGroup) Close() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	return dg.close()
}

func (dg *ConnftGroup) mappings() []dataplane.Mapping {
	return slices.Collect(maps.Values(dg.mappingsBySrc))
}

func (dg *ConnftGroup) addMappings(mappings []dataplane.Mapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// check for conflicts
	for i, m := range mappings {
		if err := m.Validate(); err != nil {
			return err
		}
		for j := 0; j < i; j++ {
			x := mappings[j]
			if m.Overlaps(&x) {
				return fmt.Errorf(
					"new mapping (proto=%s,src=%s,dst=%s) overlaps with new mapping (proto=%s,src=%s,dst=%s)",
					m.Protocol, m.Source, m.Destination,
					x.Protocol, x.Source, x.Destination,
				)
			}
		}
		for currGroupName, currGroup := range dg.dplane.groups {
			for _, x := range currGroup.mappingsBySrc {
				if m.Overlaps(&x) {
					return fmt.Errorf(
						"new mapping (proto=%s,src=%s,dst=%s) overlaps with %q: (proto=%s,src=%s,dst=%s)",
						m.Protocol, m.Source, m.Destination,
						currGroupName, x.Protocol, x.Source, x.Destination,
					)
				}
			}
		}
	}

	// ensure DNAT is actiavted immediately if needed
	if dg.enabled {
		err := dg.ensureDNATAdded(mappings)
		if err != nil {
			return err
		}
	}

	for _, m := range mappings {
		dg.mappingsBySrc[dnatKey{m.Source, m.Protocol}] = m
		dg.mappingsByDst[dnatKey{m.Destination, m.Protocol}] = m
		dg.ensureTimeoutSet(m.Protocol, m.Source, m.Timeout)
	}

	return nil
}

func (dg *ConnftGroup) delMappings(mappings []dataplane.Mapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// check if the mappings already exist in this group
	// the comparison here must be exact; e.g. [::]:443 must be in the mappings exactly to remove it
	for _, m := range mappings {
		if err := m.Validate(); err != nil {
			return err
		}
		x, exists := dg.mappingsBySrc[dnatKey{m.Source, m.Protocol}]
		if !exists {
			return dataplane.ErrNoSuchMapping
		}
		if x.Destination != m.Destination {
			return dataplane.ErrNoSuchMapping
		}
	}

	// remove from nftables
	if dg.enabled {
		err := dg.ensureDNATDeleted(mappings)
		if err != nil {
			return err
		}
	}

	// remove mappings from the group
	for _, m := range mappings {
		delete(dg.mappingsBySrc, dnatKey{m.Source, m.Protocol})
		delete(dg.mappingsByDst, dnatKey{m.Destination, m.Protocol})
		dg.ensureTimeoutDeleted(m.Protocol, m.Source)
	}
	return nil
}

func (dg *ConnftGroup) clearMappings() error {
	// clear from nftables
	mappings := dg.mappings()
	return dg.delMappings(mappings)
	// FYI: group remains enabled or disabled here
}

func (dg *ConnftGroup) close() error {
	if dg.closed {
		return nil
	}
	err := dg.clearMappings()
	dg.closed = true
	delete(dg.dplane.groups, dg.name)
	return err
}
