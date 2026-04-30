package connft

import (
	"maps"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"
	"github.com/ti-mo/conntrack"
)

var StatusDNATDone = conntrack.NewFilter().Status(conntrack.StatusDstNATDone)

// StaleGroups polls conntrack for stale entries. Returns dataplane.ErrClosed.
func (d *Connft) StaleGroups() ([]dataplane.Group, error) {
	d.lock.RLock()
	if d.closed {
		d.lock.RUnlock()
		return nil, dataplane.ErrClosed
	}
	if len(d.groups) == 0 {
		d.lock.RUnlock()
		return make([]dataplane.Group, 0), nil
	}
	d.lock.RUnlock()

	flows, err := d.ct.DumpFilter(StatusDNATDone, nil)
	defer clear(flows)

	if err != nil {
		return nil, err
	}

	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return nil, dataplane.ErrClosed
	}
	if len(d.groups) == 0 {
		return make([]dataplane.Group, 0), nil
	}

	notSeen := maps.Clone(d.groups)
	defer clear(notSeen)

	for i := range flows {
		if len(notSeen) == 0 {
			break
		}
		flow := &flows[i]
		flowDst := netip.AddrPortFrom(
			flow.TupleReply.IP.SourceAddress,
			flow.TupleReply.Proto.SourcePort,
		)
		flowProto := flow.TupleReply.Proto.Protocol

		for groupName, group := range notSeen {
			_, exists := group.mappingsByDst[dnatKey{flowDst, config.Protocol(flowProto)}]
			if exists {
				delete(notSeen, groupName)
			}
		}
	}

	result := make([]dataplane.Group, 0, len(notSeen))
	for _, group := range notSeen {
		result = append(result, group)
	}
	return result, err
}
