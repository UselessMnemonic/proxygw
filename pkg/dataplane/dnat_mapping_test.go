package dataplane

import (
	"net/netip"
	"proxygw/pkg/config"
	"testing"
)

func TestDNATMappingOverlaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  DNATMapping
		right DNATMapping
		want  bool
	}{
		{
			name:  "different protocols do not overlap",
			left:  mapping("tcp", "0.0.0.0:443", "10.0.0.10:8443"),
			right: mapping("udp", "0.0.0.0:443", "10.0.0.10:8443"),
			want:  false,
		},
		{
			name:  "different ip families do not overlap",
			left:  mapping("tcp", "0.0.0.0:443", "10.0.0.10:8443"),
			right: mapping("tcp", "[::]:443", "[2001:db8::10]:8443"),
			want:  false,
		},
		{
			name:  "reused destination overlaps",
			left:  mapping("tcp", "10.0.0.5:443", "10.0.0.10:8443"),
			right: mapping("tcp", "10.0.0.6:443", "10.0.0.10:8443"),
			want:  true,
		},
		{
			name:  "reused source overlaps",
			left:  mapping("tcp", "10.0.0.5:443", "10.0.0.10:8443"),
			right: mapping("tcp", "10.0.0.5:443", "10.0.0.11:8443"),
			want:  true,
		},
		{
			name:  "destination matching other source overlaps",
			left:  mapping("tcp", "10.0.0.5:443", "10.0.0.10:8443"),
			right: mapping("tcp", "10.0.0.10:8443", "10.0.0.11:9443"),
			want:  true,
		},
		{
			name:  "different explicit sources with same port do not overlap",
			left:  mapping("tcp", "10.0.0.5:443", "10.0.0.10:8443"),
			right: mapping("tcp", "10.0.0.6:443", "10.0.0.11:9443"),
			want:  false,
		},
		{
			name:  "ipv4 wildcard owns ipv4 port",
			left:  mapping("tcp", "0.0.0.0:443", "10.0.0.10:8443"),
			right: mapping("tcp", "10.0.0.6:443", "10.0.0.11:9443"),
			want:  true,
		},
		{
			name:  "ipv6 wildcard owns ipv6 port",
			left:  mapping("tcp", "[::]:443", "[2001:db8::10]:8443"),
			right: mapping("tcp", "[2001:db8::6]:443", "[2001:db8::11]:9443"),
			want:  true,
		},
		{
			name:  "ipv6 wildcard does not own ipv4 port",
			left:  mapping("tcp", "[::]:443", "[2001:db8::10]:8443"),
			right: mapping("tcp", "10.0.0.6:443", "10.0.0.11:9443"),
			want:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.left.Overlaps(&tc.right)
			if got != tc.want {
				t.Fatalf("Overlaps() = %v, want %v", got, tc.want)
			}

			gotReverse := tc.right.Overlaps(&tc.left)
			if gotReverse != tc.want {
				t.Fatalf("reverse Overlaps() = %v, want %v", gotReverse, tc.want)
			}
		})
	}
}

func mapping(protocol, source, destination string) DNATMapping {
	src := netip.MustParseAddrPort(source)
	dst := netip.MustParseAddrPort(destination)

	var proto config.Protocol
	switch protocol {
	case "tcp":
		proto = config.ProtocolTCP
	case "udp":
		proto = config.ProtocolUDP
	default:
		panic("unsupported protocol in test")
	}

	return DNATMapping{
		Source:      src,
		Destination: dst,
		Protocol:    proto,
	}
}
