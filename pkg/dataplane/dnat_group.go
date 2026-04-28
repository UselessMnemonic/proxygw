package dataplane

import (
	"fmt"
	"maps"
	"net/netip"
	"proxygw/pkg/config"
	"slices"
	"time"

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

// DNATGroup collects the mappings for one target. Enable and Disable switch the
// whole group in and out of the live dataplane.
type DNATGroup struct {
	dplane        *Dataplane
	name          string
	mappingsBySrc map[dnatKey]DNATMapping
	flowInfoBySrc map[dnatKey]flowInfo
	timeouts      chan DNATGroupTimeoutEvent
	lastSeen      time.Time
	ttl           config.TTL
	enabled       bool
	closed        bool
}

// NewDNATGroup reserves a mapping group with the given name.
func (d *Dataplane) NewDNATGroup(name string, ttl config.TTL) (*DNATGroup, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return nil, ErrClosed
	}
	if _, exists := d.dnatGroups[name]; exists {
		return nil, ErrGroupAlreadyRegistered
	}

	dg := &DNATGroup{
		dplane:        d,
		name:          name,
		mappingsBySrc: make(map[dnatKey]DNATMapping),
		flowInfoBySrc: make(map[dnatKey]flowInfo),
		timeouts:      make(chan DNATGroupTimeoutEvent, 10),
		lastSeen:      time.Now(),
		ttl:           ttl,
		enabled:       false,
		closed:        false,
	}
	d.dnatGroups[name] = dg
	return dg, nil
}

// Name returns the group name.
func (dg *DNATGroup) Name() string {
	return dg.name
}

// Timeout returns idle notifications for the group.
func (dg *DNATGroup) Timeout() <-chan DNATGroupTimeoutEvent {
	return dg.timeouts
}

// IsEnabled reports whether this group's mappings are currently installed.
func (dg *DNATGroup) IsEnabled() bool {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	return dg.enabled
}

// GroupTimeout returns the idle timeout used to decide when the whole target
// can be drained.
func (dg *DNATGroup) GroupTimeout() config.TTL {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	return dg.ttl
}

// SetGroupTimeout updates the idle timeout used for future drain decisions.
func (dg *DNATGroup) SetGroupTimeout(ttl config.TTL) {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	dg.ttl = ttl
}

// FlowTimeout returns the conntrack timeout configured for a source/protocol
// mapping.
func (dg *DNATGroup) FlowTimeout(source netip.AddrPort, protocol config.Protocol) (config.TTL, error) {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	m, exists := dg.mappingsBySrc[dnatKey{source, protocol}]
	if !exists {
		return 0, fmt.Errorf("source not mapped: (%s) %s", protocol, source)
	}
	return m.FlowTimeout, nil
}

// SetFlowTimeout changes the conntrack timeout for an existing mapping.
func (dg *DNATGroup) SetFlowTimeout(source netip.AddrPort, protocol config.Protocol, ttl config.TTL) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	m, exists := dg.mappingsBySrc[dnatKey{source, protocol}]
	if !exists {
		return fmt.Errorf("source not mapped: (%s) %s", protocol, source)
	}
	if m.FlowTimeout == ttl {
		return nil
	}
	if err := dg.ensureTimeoutSet(protocol, source, ttl); err != nil {
		return err
	}
	m.FlowTimeout = ttl
	dg.mappingsBySrc[dnatKey{source, protocol}] = m
	return nil
}

// Mappings returns a snapshot of this group's mappings. The order is not
// stable.
func (dg *DNATGroup) Mappings() []DNATMapping {
	dg.dplane.lock.RLock()
	defer dg.dplane.lock.RUnlock()
	return dg.mappings()
}

// AddMappings adds one or more non-overlapping mappings to the group.
func (dg *DNATGroup) AddMappings(mappings ...DNATMapping) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	return dg.addMappings(mappings)
}

// DelMappings removes exact mappings from the group.
func (dg *DNATGroup) DelMappings(mappings ...DNATMapping) error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	return dg.delMappings(mappings)
}

// ClearMappings removes every mapping from the group.
func (dg *DNATGroup) ClearMappings() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	return dg.clearMappings()
}

// Enable installs this group's mappings so traffic can be forwarded.
func (dg *DNATGroup) Enable() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
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

// Disable removes this group's mappings from the live dataplane.
func (dg *DNATGroup) Disable() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
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

// Close evicts this group from its parent Dataplane and renders this group unusable.
func (dg *DNATGroup) Close() error {
	dg.dplane.lock.Lock()
	defer dg.dplane.lock.Unlock()
	return dg.close()
}

func (dg *DNATGroup) mappings() []DNATMapping {
	return slices.Collect(maps.Values(dg.mappingsBySrc))
}

func (dg *DNATGroup) addMappings(mappings []DNATMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// check if the mappings already exist in existing groups
	for _, m := range mappings {
		if err := m.Validate(); err != nil {
			return err
		}
		for currGroupName, currGroup := range dg.dplane.dnatGroups {
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

	// update nftables
	if dg.enabled {
		err := dg.ensureDNATAdded(mappings)
		if err != nil {
			return err
		}
	}

	// update mappings
	for _, m := range mappings {
		dg.mappingsBySrc[dnatKey{m.Source, m.Protocol}] = m
		dg.ensureTimeoutSet(m.Protocol, m.Source, m.FlowTimeout)
	}

	return nil
}

func (dg *DNATGroup) delMappings(mappings []DNATMapping) error {
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
			return fmt.Errorf("source not mapped: %s", m.Source)
		}
		if x.Destination != m.Destination {
			return fmt.Errorf("destination not mapped: %s", m.Destination)
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
		dg.ensureTimeoutDeleted(m.Protocol, m.Source)
	}
	return nil
}

func (dg *DNATGroup) clearMappings() error {
	// clear from nftables
	mappings := dg.mappings()
	return dg.delMappings(mappings)
	// FYI: group remains enabled or disabled here
}

func (dg *DNATGroup) close() error {
	if dg.closed {
		return nil
	}
	err := dg.clearMappings()
	dg.closed = true
	delete(dg.dplane.dnatGroups, dg.name)
	return err
}
