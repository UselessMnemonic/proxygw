package dataplane

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/nftables"

	"proxygw/pkg/config"
)

func TestDataplaneLifecycle(t *testing.T) {
	t.Parallel()

	d := mustNewTestDataplane(t)

	table := mustLookupProxyGWTable(t)
	if table == nil {
		t.Fatal("expected proxygw table to exist after bring-up")
	}

	group, err := d.NewDNATGroup("lifecycle", config.TTL(30))
	if err != nil {
		t.Fatalf("NewDNATGroup() error = %v", err)
	}

	initial := DNATMapping{
		Source:      netip.MustParseAddrPort("127.0.0.1:8080"),
		Destination: netip.MustParseAddrPort("127.0.0.2:8080"),
		FlowTimeout: config.TTL(15),
		Protocol:    config.ProtocolTCP,
	}
	if err := group.AddMappings(initial); err != nil {
		t.Fatalf("AddMappings(initial) error = %v", err)
	}
	if err := group.Enable(); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if !group.IsEnabled() {
		t.Fatal("expected group to be enabled after bring-up")
	}
	if got := len(group.Mappings()); got != 1 {
		t.Fatalf("len(Mappings()) after bring-up = %d, want 1", got)
	}

	update := DNATMapping{
		Source:      netip.MustParseAddrPort("127.0.0.1:9090"),
		Destination: netip.MustParseAddrPort("127.0.0.3:9090"),
		FlowTimeout: config.TTL(20),
		Protocol:    config.ProtocolTCP,
	}
	if err := group.AddMappings(update); err != nil {
		t.Fatalf("AddMappings(update) error = %v", err)
	}
	if got := len(group.Mappings()); got != 2 {
		t.Fatalf("len(Mappings()) after update = %d, want 2", got)
	}

	d.Close()
	d.Wait()

	if d.Error() != nil {
		t.Fatalf("Error() after tear-down = %v, want nil", d.Error())
	}
	if _, err := d.NewDNATGroup("after-close", config.TTL(1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("NewDNATGroup() after Close error = %v, want %v", err, ErrClosed)
	}

	table = mustLookupProxyGWTable(t)
	if table != nil {
		t.Fatalf("expected proxygw table to be removed after tear-down, found %q", table.Name)
	}
}

func mustNewTestDataplane(t *testing.T) *Dataplane {
	t.Helper()

	d, err := New(context.Background())
	if err != nil {
		skipIfDataplaneUnsupported(t, err)
		t.Fatalf("New() error = %v", err)
	}

	t.Cleanup(func() {
		d.Close()
		d.Wait()
	})

	return d
}

func mustLookupProxyGWTable(t *testing.T) *nftables.Table {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		skipIfDataplaneUnsupported(t, err)
		t.Fatalf("nftables.New() error = %v", err)
	}

	table, err := conn.ListTableOfFamily("proxygw", nftables.TableFamilyINet)
	if err != nil {
		skipIfDataplaneUnsupported(t, err)
		t.Fatalf("ListTableOfFamily() error = %v", err)
	}
	return table
}

func skipIfDataplaneUnsupported(t *testing.T, err error) {
	t.Helper()

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "numerical result out of range") {
		t.Skipf("dataplane lifecycle test requires netfilter permissions: %v", err)
	}
}
