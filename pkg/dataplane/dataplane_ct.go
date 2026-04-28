package dataplane

import (
	"errors"
	"maps"
	"net/netip"
	"time"

	"github.com/ti-mo/conntrack"
)

const conntrackWatchdogPeriod = 60 * time.Second

type ctFilterResult struct {
	dsts      []netip.AddrPort
	err       error
	timestamp time.Time
}

func (d *Dataplane) start() {
	filterChan := make(chan ctFilterResult)

	// generates filter events
	d.wg.Go(func() {
		d.logger.Info("conntrack watchdog started", "period", conntrackWatchdogPeriod)
		filter := conntrack.NewFilter().Status(conntrack.StatusDstNATDone)
		ticker := time.NewTicker(conntrackWatchdogPeriod)
		defer func() {
			ticker.Stop()
			close(filterChan)
			d.logger.Info("conntrack watchdog stopped")
		}()
		for {
			select {
			case <-d.ctx.Done():
				return
			case timestamp := <-ticker.C:
				d.logger.Debug("conntrack watchdog tick", "timestamp", timestamp)
				flows, err := d.ct.DumpFilter(filter, nil)
				result := ctFilterResult{make([]netip.AddrPort, 0, len(flows)), err, timestamp}
				if err == nil {
					for _, f := range flows {
						dst := netip.AddrPortFrom(
							f.TupleMaster.IP.DestinationAddress,
							f.TupleMaster.Proto.DestinationPort,
						)
						result.dsts = append(result.dsts, dst)
					}
					clear(flows)
				}
				select {
				case <-d.ctx.Done():
					return
				case filterChan <- result:
					continue
				}
			}
		}
	})

	// process filter events
	d.wg.Go(func() {
		d.logger.Info("event processor started")
		defer func() {
			d.teardown()
			if err := d.Error(); err != nil {
				d.logger.Error("event processor stopped", "err", err)
				return
			}
			d.logger.Info("event processor stopped")
		}()
		for {
			select {
			case <-d.ctx.Done():
				return
			case result, ok := <-filterChan:
				if !ok {
					return
				}
				d.processResult(result)
			}
		}
	})
}

func (d *Dataplane) teardown() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.closed = true
	groups := maps.Clone(d.dnatGroups)
	for _, g := range groups {
		d.err = errors.Join(d.err, g.close())
	}
	d.err = errors.Join(d.err, d.ensureTableDeleted())
	d.ct.Close()
}

func (d *Dataplane) processResult(result ctFilterResult) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if result.err != nil {
		d.err = result.err
		d.logger.Error("conntrack watchdog failed", "err", result.err)
		return
	}

	notSeen := maps.Clone(d.dnatGroups)
	for groupName, group := range d.dnatGroups {
		if len(notSeen) == 0 {
			break
		}
		mappings := group.mappings()
		// match each known destination to its dnat group
	forFlow:
		for _, dst := range result.dsts {
			for _, m := range mappings {
				if m.Destination == dst {
					group.lastSeen = result.timestamp
					delete(notSeen, groupName)
					break forFlow
				}
			}
		}
	}

	// for the groups we did not see this round, possibly send timeout event
	for _, group := range notSeen {
		if result.timestamp.Sub(group.lastSeen) >= group.ttl.ToDuration() {
			d.logger.Info(
				"timeout candidate detected",
				"group", group.name,
				"idle_for", result.timestamp.Sub(group.lastSeen),
				"ttl", group.ttl.ToDuration(),
				"timestamp", result.timestamp,
			)
			group.timeouts <- DNATGroupTimeoutEvent{result.timestamp}
		}
	}
}
