package connft

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

var keyWildcard = nftables.MustConcatSetType(
	nftables.TypeInetProto, nftables.TypeInetService,
)
var keySpecific4 = nftables.MustConcatSetType(
	nftables.TypeInetProto, nftables.TypeIPAddr, nftables.TypeInetService,
)
var keySpecific6 = nftables.MustConcatSetType(
	nftables.TypeInetProto, nftables.TypeIP6Addr, nftables.TypeInetService,
)
var value4 = nftables.MustConcatSetType(
	nftables.TypeIPAddr, nftables.TypeInetService,
)
var value6 = nftables.MustConcatSetType(
	nftables.TypeIP6Addr, nftables.TypeInetService,
)

func (d *Connft) ensureTableAdded() error {
	// clear any old table
	existingTable, err := d.nft.ListTableOfFamily(d.name, nftables.TableFamilyINet)
	if err == nil && existingTable != nil {
		d.nft.DelTable(existingTable)
		err = d.nft.Flush()
	}
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("delete existing table %q: %w", d.name, err)
	}

	// define table
	d.table = d.nft.CreateTable(&nftables.Table{
		Name:   d.name,
		Family: nftables.TableFamilyINet,
	})

	// define specific lookup for ipv4
	d.dnatSpecific4 = &nftables.Set{
		Table:         d.table,
		Name:          "dnatSpecific4",
		IsMap:         true,
		Concatenation: true,
		KeyType:       keySpecific4,
		DataType:      value4,
	}
	if err = d.nft.AddSet(d.dnatSpecific4, nil); err != nil {
		return fmt.Errorf("define specific ipv4 dnat set: %w", err)
	}

	// define specific lookup for ipv6
	d.dnatSpecific6 = &nftables.Set{
		Table:         d.table,
		Name:          "dnatSpecific6",
		IsMap:         true,
		Concatenation: true,
		KeyType:       keySpecific6,
		DataType:      value6,
	}
	if err = d.nft.AddSet(d.dnatSpecific6, nil); err != nil {
		return fmt.Errorf("define specific ipv6 dnat set: %w", err)
	}

	// define wildcard lookup for ipv4
	d.dnatWildcard4 = &nftables.Set{
		Table:         d.table,
		Name:          "dnatWildcard4",
		IsMap:         true,
		Concatenation: true,
		KeyType:       keyWildcard,
		DataType:      value4,
	}
	if err = d.nft.AddSet(d.dnatWildcard4, nil); err != nil {
		return fmt.Errorf("define wildcard ipv4 dnat set: %w", err)
	}

	// define wildcard lookup for ipv6
	d.dnatWildcard6 = &nftables.Set{
		Table:         d.table,
		Name:          "dnatWildcard6",
		IsMap:         true,
		Concatenation: true,
		KeyType:       keyWildcard,
		DataType:      value6,
	}
	if err = d.nft.AddSet(d.dnatWildcard6, nil); err != nil {
		return fmt.Errorf("define wildcard ipv6 dnat set: %w", err)
	}

	// DNAT chain for external traffic
	d.prerouteNATChain = &nftables.Chain{
		Table:    d.table,
		Name:     "prerouting",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityNATDest,
	}
	d.nft.AddChain(d.prerouteNATChain)

	// define filter chain for timeouts
	d.inputFilterChain = &nftables.Chain{
		Table:    d.table,
		Name:     "timeouts",
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
	}
	d.nft.AddChain(d.inputFilterChain)

	// submit changes for base table
	if err = d.nft.Flush(); err != nil {
		return fmt.Errorf("create table %q: %w", d.table.Name, err)
	}

	d.addDNATRules(d.prerouteNATChain)

	// submit changes for base rules
	if err = d.nft.Flush(); err != nil {
		return fmt.Errorf("create table rules for %q: %w", d.table.Name, err)
	}
	return nil
}

func (d *Connft) ensureTableDeleted() error {
	d.nft.DelTable(d.table)
	err := d.nft.Flush()
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

func (dg *ConnftGroup) ensureTimeoutSet(proto config.Protocol, addr netip.AddrPort, ttl config.TTL) error {
	if ttl == 0 {
		return dg.ensureTimeoutDeleted(proto, addr)
	}

	// update existing TTL
	if info, present := dg.flowInfoBySrc[dnatKey{addr, proto}]; present {
		ttlObj := info.timeoutObj.Obj.(*expr.CtTimeout)
		ttlObj.Policy[expr.CtStateUDPUNREPLIED] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateUDPREPLIED] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPESTABLISHED] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPFINWAIT] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPTIMEWAIT] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPCLOSEWAIT] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPLASTACK] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateTCPCLOSE] = ttl.Seconds()
		dg.dplane.nft.AddObj(info.timeoutObj)
		if err := dg.dplane.nft.Flush(); err != nil {
			return fmt.Errorf("update flow timeout object: %w", err)
		}
		return nil
	}

	var l3Protocol uint16
	if addr.Addr().Is4() {
		l3Protocol = unix.NFPROTO_IPV4
	}
	if addr.Addr().Is6() {
		l3Protocol = unix.NFPROTO_IPV6
	}
	l4Protocol := uint8(proto)

	// create new TTL
	timeoutObj := &nftables.NamedObj{
		Table: dg.dplane.table,
		Name:  encodeTimeoutObjName(proto, addr),
		Type:  nftables.ObjTypeCtTimeout,
		Obj: &expr.CtTimeout{
			L3Proto: l3Protocol,
			L4Proto: l4Protocol,
			Policy: expr.CtStatePolicyTimeout{
				expr.CtStateUDPUNREPLIED:   ttl.Seconds(),
				expr.CtStateUDPREPLIED:     ttl.Seconds(),
				expr.CtStateTCPESTABLISHED: ttl.Seconds(),
				expr.CtStateTCPFINWAIT:     ttl.Seconds(),
				expr.CtStateTCPTIMEWAIT:    ttl.Seconds(),
				expr.CtStateTCPCLOSEWAIT:   ttl.Seconds(),
				expr.CtStateTCPLASTACK:     ttl.Seconds(),
				expr.CtStateTCPCLOSE:       ttl.Seconds(),
			},
		},
	}
	dg.dplane.nft.AddObj(timeoutObj)

	timeoutRule := dg.dplane.defineTimeoutRule(l4Protocol, addr, timeoutObj)
	dg.dplane.nft.AddRule(timeoutRule)

	if err := dg.dplane.nft.Flush(); err != nil {
		return fmt.Errorf("create ttl rules: %w", err)
	}
	dg.flowInfoBySrc[dnatKey{addr, proto}] = flowInfo{timeoutObj, timeoutRule}
	return nil
}

func (dg *ConnftGroup) ensureTimeoutDeleted(proto config.Protocol, addr netip.AddrPort) error {
	info, present := dg.flowInfoBySrc[dnatKey{addr, proto}]
	if !present {
		return nil
	}
	if info.timeoutRule != nil {
		if err := dg.dplane.nft.DelRule(info.timeoutRule); err != nil && !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("prepare delete ttl rule: %w", err)
		}
	}
	dg.dplane.nft.DeleteObject(info.timeoutObj)
	if err := dg.dplane.nft.Flush(); err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("delete ttl rule: %w", err)
	}
	delete(dg.flowInfoBySrc, dnatKey{addr, proto})
	return nil
}

func (dg *ConnftGroup) ensureDNATAdded(mappings []dataplane.Mapping) error {
	if len(mappings) == 0 {
		return nil
	}
	for _, m := range mappings {
		elem := compileSetElement(m)
		if m.Source.Addr().Is6() {
			set := dg.dplane.dnatSpecific6
			if m.Source.Addr().IsUnspecified() {
				set = dg.dplane.dnatWildcard6
			}
			if err := dg.dplane.nft.SetAddElements(set, []nftables.SetElement{elem}); err != nil {
				return fmt.Errorf("prepare add elements: %w", err)
			}
		}
		if m.Source.Addr().Is4() {
			set := dg.dplane.dnatSpecific4
			if m.Source.Addr().IsUnspecified() {
				set = dg.dplane.dnatWildcard4
			}
			if err := dg.dplane.nft.SetAddElements(set, []nftables.SetElement{elem}); err != nil {
				return fmt.Errorf("prepare add elements: %w", err)
			}
		}
	}
	if err := dg.dplane.nft.Flush(); err != nil {
		return fmt.Errorf("add elements: %w", err)
	}
	return nil
}

func (dg *ConnftGroup) ensureDNATDeleted(mappings []dataplane.Mapping) error {
	if len(mappings) == 0 {
		return nil
	}
	for _, m := range mappings {
		elem := compileSetElement(m)
		if m.Source.Addr().Is6() {
			set := dg.dplane.dnatSpecific6
			if m.Source.Addr().IsUnspecified() {
				set = dg.dplane.dnatWildcard6
			}
			if err := dg.dplane.nft.SetDeleteElements(set, []nftables.SetElement{elem}); err != nil {
				return fmt.Errorf("prepare del elements: %w", err)
			}
		}
		if m.Source.Addr().Is4() {
			set := dg.dplane.dnatSpecific4
			if m.Source.Addr().IsUnspecified() {
				set = dg.dplane.dnatWildcard4
			}
			if err := dg.dplane.nft.SetDeleteElements(set, []nftables.SetElement{elem}); err != nil {
				return fmt.Errorf("prepare del elements: %w", err)
			}
		}
	}
	if err := dg.dplane.nft.Flush(); err != nil {
		return fmt.Errorf("remove elements: %w", err)
	}
	return nil
}
