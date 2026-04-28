package dataplane

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/UselessMnemonic/proxygw/pkg/config"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"
)

func TestDataplaneLifecycle(t *testing.T) {
	t.Parallel()

	tableName := testDataplaneName(t)
	d := mustNewTestDataplane(t, tableName)

	table := lookupProxyGWTable(t, tableName)
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
	waitForProxyGWTableDeleted(t, tableName)

	if d.Error() != nil {
		t.Fatalf("Error() after tear-down = %v, want nil", d.Error())
	}
	if _, err := d.NewDNATGroup("after-close", config.TTL(1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("NewDNATGroup() after Close error = %v, want %v", err, ErrClosed)
	}

	table = lookupProxyGWTable(t, tableName)
	if table != nil {
		t.Fatalf("expected proxygw table to be removed after tear-down, found %q", table.Name)
	}
}

func mustNewTestDataplane(t *testing.T, tableName string) *Dataplane {
	t.Helper()

	d, err := New(context.Background(), tableName)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Cleanup(func() {
		d.Close()
		d.Wait()
		waitForProxyGWTableDeleted(t, tableName)
	})

	return d
}

func lookupProxyGWTable(t *testing.T, tableName string) *nftables.Table {
	t.Helper()

	conn, err := nftables.New()
	if err != nil {
		t.Fatalf("nftables.New() error = %v", err)
	}

	table, err := conn.ListTableOfFamily(tableName, nftables.TableFamilyINet)
	if err == nil {
		return table
	}
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	t.Fatalf("ListTableOfFamily() error = %v", err)
	return table
}

func waitForProxyGWTableDeleted(t *testing.T, tableName string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if lookupProxyGWTable(t, tableName) == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	table := lookupProxyGWTable(t, tableName)
	if table != nil {
		t.Fatalf("timed out waiting for proxygw table to be deleted; found %q", table.Name)
	}
}

func testDataplaneName(t *testing.T) string {
	t.Helper()

	name := strings.ToLower(t.Name())
	name = strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(name)
	return fmt.Sprintf("proxygw_%s", name)
}
