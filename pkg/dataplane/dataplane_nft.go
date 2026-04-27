package dataplane

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"net/netip"
	"proxygw/pkg/config"

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

func (d *Dataplane) ensureTableAdded() error {
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
	if err := d.nft.AddSet(d.dnatSpecific4, nil); err != nil {
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
	if err := d.nft.AddSet(d.dnatSpecific6, nil); err != nil {
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
	if err := d.nft.AddSet(d.dnatWildcard4, nil); err != nil {
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
	if err := d.nft.AddSet(d.dnatWildcard6, nil); err != nil {
		return fmt.Errorf("define wildcard ipv6 dnat set: %w", err)
	}

	// define NAT chain
	d.prerouteNATChain = &nftables.Chain{
		Table:    d.table,
		Name:     "prerouting",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityNATDest,
	}
	d.nft.AddChain(d.prerouteNATChain)

	// define filter chain
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

	// define wildcard forwarding rules for ipv4
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: d.prerouteNATChain,
		Exprs: []expr.Any{
			// meta nfproto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			// reg 1 == ipv4
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			// meta l4proto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			// load th dport -> reg 2
			&expr.Payload{
				DestRegister: unix.NFT_REG_2,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			// lookup key (l4proto . th dport) in @dnatWildcard4, write value to reg 1+
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatWildcard4.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatWildcard4.Name,
			},
			// dnat ip to <mapped addr> : <mapped port>
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG_2,
				Specified:   true,
			},
		},
	})

	// define wildcard forwarding rules for ipv6
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: d.prerouteNATChain,
		Exprs: []expr.Any{
			// meta nfproto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			// reg 1 == ipv6
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV6},
			},
			// meta l4proto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			// load th dport -> reg 2
			&expr.Payload{
				DestRegister: unix.NFT_REG_2,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			// lookup key (l4proto . th dport) in @dnatWildcard6, write value to reg 1+
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatWildcard6.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatWildcard6.Name,
			},
			// dnat ip6 to <mapped addr> : <mapped port>
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV6,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_04,
				Specified:   true,
			},
		},
	})

	// define specific forwarding rules for ipv4
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: d.prerouteNATChain,
		Exprs: []expr.Any{
			// meta nfproto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			// reg 1 == ipv4
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			// meta l4proto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			// load ip daddr -> reg 2
			&expr.Payload{
				DestRegister: unix.NFT_REG_2,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16,
				Len:          4,
			},
			// load th dport -> reg 3
			&expr.Payload{
				DestRegister: unix.NFT_REG_3,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			// lookup key (l4proto . ip daddr . th dport) in @dnatSpecific4, write value to reg 1+
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatSpecific4.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatSpecific4.Name,
			},
			// dnat ip to <mapped addr> : <mapped port>
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG_2,
				Specified:   true,
			},
		},
	})

	// define forwarding rules for ipv6
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: d.prerouteNATChain,
		Exprs: []expr.Any{
			// meta nfproto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG32_00,
			},
			// reg 1 == ipv6
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG32_00,
				Data:     []byte{unix.NFPROTO_IPV6},
			},
			// meta l4proto -> reg 1
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG32_00,
			},
			// load ip6 daddr -> reg 2
			&expr.Payload{
				DestRegister: unix.NFT_REG32_01,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24,
				Len:          16,
			},
			// load th dport -> reg 6
			&expr.Payload{
				DestRegister: unix.NFT_REG32_05,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			// lookup key (l4proto . ip6 daddr . th dport) in @dnatSpecific6, write value to reg 1+
			&expr.Lookup{
				SourceRegister: unix.NFT_REG32_00,
				SetID:          d.dnatSpecific6.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatSpecific6.Name,
			},
			// dnat ip6 to <mapped addr> : <mapped port>
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV6,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_04,
				Specified:   true,
			},
		},
	})

	// submit changes for base rules
	if err = d.nft.Flush(); err != nil {
		return fmt.Errorf("create table rules for %q: %w", d.table.Name, err)
	}
	return nil
}

func (d *Dataplane) ensureTableDeleted() error {
	d.nft.DelTable(d.table)
	err := d.nft.Flush()
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

func (dg *DNATGroup) ensureTimeoutSet(proto config.Protocol, addr netip.AddrPort, ttl config.TTL) error {
	if ttl == 0 {
		return dg.ensureTimeoutDeleted(proto, addr)
	}

	// update existing TTL
	if info, present := dg.flowInfoBySrc[dnatKey{addr, proto}]; present {
		ttlObj := info.timeoutObj.Obj.(*expr.CtTimeout)
		ttlObj.Policy[expr.CtStateUDPUNREPLIED] = ttl.Seconds()
		ttlObj.Policy[expr.CtStateUDPREPLIED] = ttl.Seconds()
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
	info := flowInfo{}
	info.timeoutObj = &nftables.NamedObj{
		Table: dg.dplane.table,
		Name:  encodeTimeoutObjName(proto, addr),
		Type:  nftables.ObjTypeCtTimeout,
		Obj: &expr.CtTimeout{
			L3Proto: l3Protocol,
			L4Proto: l4Protocol,
			Policy: expr.CtStatePolicyTimeout{
				expr.CtStateUDPUNREPLIED: ttl.Seconds(),
				expr.CtStateUDPREPLIED:   ttl.Seconds(),
			},
		},
	}
	dg.dplane.nft.AddObj(info.timeoutObj)

	info.timeoutRule = &nftables.Rule{
		Table: dg.dplane.table,
		Chain: dg.dplane.inputFilterChain,
	}

	// create rule for ipv6
	if addr.Addr().Is6() {
		if !addr.Addr().IsUnspecified() {
			info.timeoutRule.Exprs = []expr.Any{
				// meta l4proto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == configured protocol (tcp/udp)
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				// meta nfproto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == ipv6 for this frontend
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV6, 0},
				},
				// load ip6 daddr -> reg 2
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       24,
					Len:          16,
				},
				// reg 2 == frontend listen IPv6 address
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     addr.Addr().AsSlice(),
				},
				// load th dport -> reg 6
				&expr.Payload{
					DestRegister: unix.NFT_REG_3,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				// reg 6 == frontend listen port
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_3,
					Data:     encodePort(addr.Port()),
				},
				// apply ct timeout policy object by name
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: info.timeoutObj.Name,
				},
			}
		}
		if addr.Addr().IsUnspecified() {
			info.timeoutRule.Exprs = []expr.Any{
				// meta l4proto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == configured protocol (tcp/udp)
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				// meta nfproto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == ipv6 for this frontend
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV6, 0},
				},
				// load th dport -> reg 2
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				// reg 2 == frontend listen port
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     encodePort(addr.Port()),
				},
				// apply ct timeout policy object by name
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: info.timeoutObj.Name,
				},
			}
		}
	}

	// create rules for ipv4
	if addr.Addr().Is4() {
		if !addr.Addr().IsUnspecified() {
			info.timeoutRule.Exprs = []expr.Any{
				// meta l4proto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == configured protocol (tcp/udp)
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				// meta nfproto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == ipv4 or ipv6 family for this frontend
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV4, 0},
				},
				// load ip daddr -> reg 2
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       16,
					Len:          4,
				},
				// reg 2 == frontend listen IPv4 address
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     addr.Addr().AsSlice(),
				},
				// load th dport -> reg 3
				&expr.Payload{
					DestRegister: unix.NFT_REG_3,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				// reg 3 == frontend listen port
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_3,
					Data:     encodePort(addr.Port()),
				},
				// apply ct timeout policy object by name
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: info.timeoutObj.Name,
				},
			}
		}
		if addr.Addr().IsUnspecified() {
			info.timeoutRule.Exprs = []expr.Any{
				// meta l4proto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == configured protocol (tcp/udp)
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				// meta nfproto -> reg 1
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				// reg 1 == ipv4 for this frontend
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV4, 0},
				},
				// load th dport -> reg 2
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				// reg 2 == frontend listen port
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     encodePort(addr.Port()),
				},
				// apply ct timeout policy object by name
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: info.timeoutObj.Name,
				},
			}
		}
	}
	dg.dplane.nft.AddRule(info.timeoutRule)

	if err := dg.dplane.nft.Flush(); err != nil {
		return fmt.Errorf("create ttl rules: %w", err)
	}
	dg.flowInfoBySrc[dnatKey{addr, proto}] = info
	return nil
}

func (dg *DNATGroup) ensureTimeoutDeleted(proto config.Protocol, addr netip.AddrPort) error {
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

func (dg *DNATGroup) ensureDNATAdded(mappings []DNATMapping) error {
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

func (dg *DNATGroup) ensureDNATDeleted(mappings []DNATMapping) error {
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

func compileSetElement(mapping DNATMapping) nftables.SetElement {
	key := encodeSpecificKey(mapping.Protocol, mapping.Source)
	if mapping.Source.Addr().IsUnspecified() {
		key = encodeWildcardKey(mapping.Protocol, mapping.Source.Port())
	}

	return nftables.SetElement{
		Key: key,
		Val: encodeMapValue(mapping.Destination),
	}
}

func encodeTimeoutObjName(protocol config.Protocol, address netip.AddrPort) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte{byte(protocol)})
	_, _ = hasher.Write(address.Addr().AsSlice())
	_, _ = hasher.Write(encodePort(address.Port()))
	return fmt.Sprintf("ct_%016x", hasher.Sum64())
}

func encodeSpecificKey(protocol config.Protocol, address netip.AddrPort) []byte {
	addr := address.Addr().AsSlice()
	buf := make([]byte, 0, 4+len(addr)+4)
	buf = appendConcatField(buf, []byte{uint8(protocol)})
	buf = appendConcatField(buf, addr)
	buf = appendConcatField(buf, encodePort(address.Port()))
	return buf
}

func encodeWildcardKey(protocol config.Protocol, port uint16) []byte {
	buf := make([]byte, 0, 8)
	buf = appendConcatField(buf, []byte{uint8(protocol)})
	buf = appendConcatField(buf, encodePort(port))
	return buf
}

func encodeMapValue(address netip.AddrPort) []byte {
	addr := address.Addr().AsSlice()
	buf := make([]byte, 0, len(addr)+4)
	buf = appendConcatField(buf, addr)
	buf = appendConcatField(buf, encodePort(address.Port()))
	return buf
}

func encodePort(port uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, port)
	return buf
}

func appendConcatField(dst []byte, field []byte) []byte {
	dst = append(dst, field...)
	if rem := len(field) % 4; rem != 0 {
		dst = append(dst, make([]byte, 4-rem)...)
	}
	return dst
}
