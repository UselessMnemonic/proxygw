package connft

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"

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

	group, err := d.NewConnftGroup("lifecycle")
	if err != nil {
		t.Fatalf("NewConnftGroup() error = %v", err)
	}

	initial := dataplane.Mapping{
		Source:      netip.MustParseAddrPort("127.0.0.1:8080"),
		Destination: netip.MustParseAddrPort("127.0.0.2:8080"),
		Timeout:     config.TTL(15),
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

	update := dataplane.Mapping{
		Source:      netip.MustParseAddrPort("127.0.0.1:9090"),
		Destination: netip.MustParseAddrPort("127.0.0.3:9090"),
		Timeout:     config.TTL(20),
		Protocol:    config.ProtocolTCP,
	}
	if err := group.AddMappings(update); err != nil {
		t.Fatalf("AddMappings(update) error = %v", err)
	}
	if got := len(group.Mappings()); got != 2 {
		t.Fatalf("len(Mappings()) after update = %d, want 2", got)
	}

	if err := d.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	waitForProxyGWTableDeleted(t, tableName)

	if _, err := d.NewConnftGroup("after-close"); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("NewConnftGroup() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if _, err := d.StaleGroups(); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("StaleGroups() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.AddMappings(initial); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("AddMappings() after dataplane Close error = %v, want %v", err, dataplane.ErrClosed)
	}

	table = lookupProxyGWTable(t, tableName)
	if table != nil {
		t.Fatalf("expected proxygw table to be removed after tear-down, found %q", table.Name)
	}
}

func TestGroupRejectsOverlappingBatchAtomically(t *testing.T) {
	t.Parallel()

	tableName := testDataplaneName(t)
	d := mustNewTestDataplane(t, tableName)

	group, err := d.NewConnftGroup("batch")
	if err != nil {
		t.Fatalf("NewConnftGroup() error = %v", err)
	}

	first := testMapping("127.0.0.1:1001", "127.0.0.2:2001", config.TTL(15))
	overlapping := testMapping("127.0.0.3:1003", "127.0.0.2:2001", config.TTL(20))

	if err := group.AddMappings(first, overlapping); err == nil {
		t.Fatal("AddMappings(overlapping batch) error = nil, want error")
	}
	if got := len(group.Mappings()); got != 0 {
		t.Fatalf("len(Mappings()) after rejected batch = %d, want 0", got)
	}
}

func TestGroupOperationsAfterCloseReturnErrClosed(t *testing.T) {
	t.Parallel()

	tableName := testDataplaneName(t)
	d := mustNewTestDataplane(t, tableName)

	group, err := d.NewConnftGroup("closed")
	if err != nil {
		t.Fatalf("NewConnftGroup() error = %v", err)
	}

	mapping := testMapping("127.0.0.1:1001", "127.0.0.2:2001", config.TTL(15))
	if err := group.AddMappings(mapping); err != nil {
		t.Fatalf("AddMappings() error = %v", err)
	}
	if err := group.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := group.AddMappings(mapping); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("AddMappings() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.DelMappings(mapping); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("DelMappings() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.ClearMappings(); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("ClearMappings() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.Enable(); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("Enable() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.Disable(); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("Disable() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if _, err := group.Timeout(mapping.Protocol, mapping.Source); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("Timeout() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.SetTimeout(mapping.Protocol, mapping.Source, config.TTL(30)); !errors.Is(err, dataplane.ErrClosed) {
		t.Fatalf("SetTimeout() after Close error = %v, want %v", err, dataplane.ErrClosed)
	}
	if err := group.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}
}

func mustNewTestDataplane(t *testing.T, tableName string) *Connft {
	t.Helper()

	d, err := New(tableName)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Cleanup(func() {
		d.Close()
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

func testMapping(source, destination string, timeout config.TTL) dataplane.Mapping {
	return dataplane.Mapping{
		Source:      netip.MustParseAddrPort(source),
		Destination: netip.MustParseAddrPort(destination),
		Timeout:     timeout,
		Protocol:    config.ProtocolTCP,
	}
}
