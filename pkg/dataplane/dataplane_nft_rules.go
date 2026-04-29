package dataplane

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

func (d *Dataplane) addDNATRules(chain *nftables.Chain) {
	// Wildcard IPv4 DNAT key: meta l4proto . th dport.
	// Lookup returns mapped IPv4 address . mapped port in NFT_REG_1+.
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_01,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatWildcard4.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatWildcard4.Name,
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_01,
				Specified:   true,
			},
		},
	})

	// Wildcard IPv6 DNAT key: meta l4proto . th dport.
	// Lookup returns mapped IPv6 address . mapped port; port starts at REG32_04.
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV6},
			},
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_01,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatWildcard6.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatWildcard6.Name,
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV6,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_04,
				Specified:   true,
			},
		},
	})

	// Specific IPv4 DNAT key: meta l4proto . ip daddr . th dport.
	// ip daddr starts at REG32_01; th dport follows at REG32_02.
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG_1,
				Data:     []byte{unix.NFPROTO_IPV4},
			},
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG_1,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_01,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16,
				Len:          4,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_02,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Lookup{
				SourceRegister: unix.NFT_REG_1,
				SetID:          d.dnatSpecific4.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatSpecific4.Name,
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_01,
				Specified:   true,
			},
		},
	})

	// Specific IPv6 DNAT key: meta l4proto . ip6 daddr . th dport.
	// ip6 daddr occupies REG32_01..REG32_04; th dport follows at REG32_05.
	d.nft.AddRule(&nftables.Rule{
		Table: d.table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyNFPROTO,
				Register: unix.NFT_REG32_00,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: unix.NFT_REG32_00,
				Data:     []byte{unix.NFPROTO_IPV6},
			},
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: unix.NFT_REG32_00,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_01,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24,
				Len:          16,
			},
			&expr.Payload{
				DestRegister: unix.NFT_REG32_05,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Lookup{
				SourceRegister: unix.NFT_REG32_00,
				SetID:          d.dnatSpecific6.ID,
				DestRegister:   unix.NFT_REG_1,
				IsDestRegSet:   true,
				SetName:        d.dnatSpecific6.Name,
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      unix.NFPROTO_IPV6,
				RegAddrMin:  unix.NFT_REG_1,
				RegProtoMin: unix.NFT_REG32_04,
				Specified:   true,
			},
		},
	})
}

func (d *Dataplane) defineTimeoutRule(l4Protocol uint8, addr netip.AddrPort, timeoutObj *nftables.NamedObj) *nftables.Rule {
	timeoutRule := &nftables.Rule{
		Table: d.table,
		Chain: d.inputFilterChain,
	}

	// Timeout rules run after DNAT and attach the frontend's named ct timeout.
	if addr.Addr().Is6() {
		if !addr.Addr().IsUnspecified() {
			// Specific IPv6 match: l4proto . nfproto . ip6 daddr . th dport.
			timeoutRule.Exprs = []expr.Any{
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV6},
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       24,
					Len:          16,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     addr.Addr().AsSlice(),
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_3,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_3,
					Data:     encodePort(addr.Port()),
				},
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: timeoutObj.Name,
				},
			}
		}
		if addr.Addr().IsUnspecified() {
			// Wildcard IPv6 match: l4proto . nfproto . th dport.
			timeoutRule.Exprs = []expr.Any{
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV6},
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     encodePort(addr.Port()),
				},
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: timeoutObj.Name,
				},
			}
		}
	}

	if addr.Addr().Is4() {
		if !addr.Addr().IsUnspecified() {
			// Specific IPv4 match: l4proto . nfproto . ip daddr . th dport.
			timeoutRule.Exprs = []expr.Any{
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV4},
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       16,
					Len:          4,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     addr.Addr().AsSlice(),
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_3,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_3,
					Data:     encodePort(addr.Port()),
				},
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: timeoutObj.Name,
				},
			}
		}
		if addr.Addr().IsUnspecified() {
			// Wildcard IPv4 match: l4proto . nfproto . th dport.
			timeoutRule.Exprs = []expr.Any{
				&expr.Meta{
					Key:      expr.MetaKeyL4PROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{l4Protocol},
				},
				&expr.Meta{
					Key:      expr.MetaKeyNFPROTO,
					Register: unix.NFT_REG_1,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_1,
					Data:     []byte{unix.NFPROTO_IPV4},
				},
				&expr.Payload{
					DestRegister: unix.NFT_REG_2,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: unix.NFT_REG_2,
					Data:     encodePort(addr.Port()),
				},
				&expr.Objref{
					Type: int(nftables.ObjTypeCtTimeout),
					Name: timeoutObj.Name,
				},
			}
		}
	}
	return timeoutRule
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
