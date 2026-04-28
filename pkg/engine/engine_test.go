package engine

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

func TestNew(t *testing.T) {
	e := mustNewTestEngine(t)

	if e == nil {
		t.Fatal("New() returned nil engine")
	}
	if e.Closed() {
		t.Fatal("Closed() = true, want false")
	}
}

func TestNewTarget(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := config.NamespaceReference{
		Namespace: "test",
		Name:      "target",
	}
	if err := e.AddTargetKind(targetKind.String(), newStubTargetHandler); err != nil {
		t.Fatalf("AddTargetKind() error = %v", err)
	}

	targetCfg := config.Target{
		Name:        "backend",
		Kind:        targetKind,
		IdleTimeout: config.TTL(30),
		Endpoints: []config.TargetEndpoint{
			{
				Name:     "http",
				Protocol: config.ProtocolTCP,
				Address:  netip.MustParseAddrPort("127.0.0.2:8080"),
			},
		},
	}

	got, err := e.NewTarget(targetCfg)
	if err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	if got == nil {
		t.Fatal("NewTarget() returned nil target")
	}
	if got.Name() != targetCfg.Name {
		t.Fatalf("Name() = %q, want %q", got.Name(), targetCfg.Name)
	}
	if got.Kind() != targetKind.String() {
		t.Fatalf("Kind() = %q, want %q", got.Kind(), targetKind.String())
	}
	if _, ok := got.Endpoint("http"); !ok {
		t.Fatal(`Endpoint("http") = missing, want present`)
	}
	if e.GetTarget(targetCfg.Name) != got {
		t.Fatal("GetTarget() did not return created target")
	}
	if len(e.Targets()) != 1 {
		t.Fatalf("len(Targets()) = %d, want 1", len(e.Targets()))
	}
}

func TestNewFrontend(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := config.NamespaceReference{
		Namespace: "test",
		Name:      "target",
	}
	frontendKind := config.NamespaceReference{
		Namespace: "test",
		Name:      "frontend",
	}
	if err := e.AddTargetKind(targetKind.String(), newStubTargetHandler); err != nil {
		t.Fatalf("AddTargetKind() error = %v", err)
	}
	if err := e.AddFrontendKind(frontendKind.String(), newStubFrontendHandler); err != nil {
		t.Fatalf("AddFrontendKind() error = %v", err)
	}

	targetCfg := config.Target{
		Name:        "backend",
		Kind:        targetKind,
		IdleTimeout: config.TTL(30),
		Endpoints: []config.TargetEndpoint{
			{
				Name:     "http",
				Protocol: config.ProtocolTCP,
				Address:  netip.MustParseAddrPort("127.0.0.2:8080"),
			},
		},
	}
	if _, err := e.NewTarget(targetCfg); err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	frontendCfg := config.Frontend{
		Name:        "listener",
		Kind:        frontendKind,
		Protocol:    config.ProtocolTCP,
		Listen:      netip.MustParseAddrPort("127.0.0.1:8080"),
		FlowTimeout: config.TTL(15),
		Endpoint: config.NamespaceReference{
			Namespace: "backend",
			Name:      "http",
		},
	}

	got, err := e.NewFrontend(frontendCfg)
	if err != nil {
		t.Fatalf("NewFrontend() error = %v", err)
	}

	if got == nil {
		t.Fatal("NewFrontend() returned nil frontend")
	}
	if got.Name() != frontendCfg.Name {
		t.Fatalf("Name() = %q, want %q", got.Name(), frontendCfg.Name)
	}
	if got.Kind() != frontendKind.String() {
		t.Fatalf("Kind() = %q, want %q", got.Kind(), frontendKind.String())
	}
	if got.Target() == nil || got.Target().Name() != "backend" {
		t.Fatal(`Target() = nil or wrong target, want "backend"`)
	}
	if got.Endpoint().Name != "http" {
		t.Fatalf("Endpoint().Name = %q, want %q", got.Endpoint().Name, "http")
	}
	if e.GetFrontend(frontendCfg.Name) != got {
		t.Fatal("GetFrontend() did not return created frontend")
	}
	if len(e.Frontends()) != 1 {
		t.Fatalf("len(Frontends()) = %d, want 1", len(e.Frontends()))
	}
}

func mustNewTestEngine(t *testing.T) *Engine {
	t.Helper()

	tableName := testDataplaneName(t)
	e, err := New(context.Background(), tableName)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Cleanup(func() {
		e.Close()
		e.Wait()
		waitForProxyGWTableDeleted(t, tableName)
	})

	return e
}

type stubTargetHandler struct{}

func newStubTargetHandler(name string, options map[string]any) (target.Handler, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return stubTargetHandler{}, nil
}

func (stubTargetHandler) Warm() error  { return nil }
func (stubTargetHandler) Drain() error { return nil }
func (stubTargetHandler) Close() error { return nil }

type stubFrontendHandler struct {
	shouldWarm chan struct{}
}

func newStubFrontendHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return &stubFrontendHandler{
		shouldWarm: make(chan struct{}),
	}, nil
}

func (h *stubFrontendHandler) Start() error                { return nil }
func (h *stubFrontendHandler) Stop() error                 { return nil }
func (h *stubFrontendHandler) Close() error                { return nil }
func (h *stubFrontendHandler) ShouldWarm() <-chan struct{} { return h.shouldWarm }

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
	return nil
}

func testDataplaneName(t *testing.T) string {
	t.Helper()

	name := strings.ToLower(t.Name())
	name = strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(name)
	return fmt.Sprintf("proxygw_%s", name)
}
