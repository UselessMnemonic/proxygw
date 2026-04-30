package dataplane

import (
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
)

// Group is a logical group of 1:1 DNAT mappings.
type Group interface {
	// Enable causes the underlying dataplane to enable DNAT for all mappings.
	// This operation is atomic, either all mappings are enabled or none are.
	// Returns ErrClosed.
	Enable() error
	// Disable causes the underlying dataplane to disable DNAT for all mappings.
	// This operation is atomic, either all mappings are disabled or none are.
	// Returns ErrClosed.
	Disable() error
	// AddMappings defines the given mappings in this group.
	// This operation is atomic, either all mappings are added or none are.
	// If the group is enabled, then the underlying dataplane
	// enables DNAT for the added mappings. Returns ErrClosed.
	AddMappings(mapping ...Mapping) error
	// DelMappings removes the given mappings in this group.
	// This operation is atomic, either all mappings are removed or none are.
	// If the group is enabled, then the underlying dataplane
	// disables DNAT for the removed mappings. Returns ErrClosed or
	// ErrNoSuchMapping.
	DelMappings(mapping ...Mapping) error
	// ClearMappings removes all given mappings in this group.
	// This operation is atomic, either all mappings are removed or none are.
	// If the group is enabled, then the underlying dataplane
	// disables DNAT for the removed mappings. Returns ErrClosed.
	ClearMappings() error
	// Timeout retrieves the flow timeout for the given source.
	// Returns ErrClosed or ErrNoSuchMapping.
	Timeout(protocol config.Protocol, source netip.AddrPort) (config.TTL, error)
	// SetTimeout updates the flow timeout for a given source.
	// Returns ErrClosed or ErrNoSuchMapping.
	SetTimeout(protocol config.Protocol, source netip.AddrPort, timeout config.TTL) error
	// Close invalidates this group, rendering it useless. It is safe to call multiple times.
	// All mappings are deleted with the same effect as DelMappings.
	// When a group is closed, all operations return ErrClosed.
	Close() error
}
