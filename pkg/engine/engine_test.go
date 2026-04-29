package engine

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
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

	targetKind := testKind("target")
	mustAddTargetKind(t, e, targetKind, newStubTargetHandler)
	targetCfg := testTargetConfig(targetKind, nil)

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

	targetKind := testKind("target")
	frontendKind := testKind("frontend")
	mustAddTargetKind(t, e, targetKind, newStubTargetHandler)
	mustAddFrontendKind(t, e, frontendKind, newStubFrontendHandler)

	targetCfg := testTargetConfig(targetKind, nil)
	if _, err := e.NewTarget(targetCfg); err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	frontendCfg := testFrontendConfig(frontendKind, nil)

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

func TestTargetCloseDoesNotWaitForHandlerClose(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("blocking-target")
	mustAddTargetKind(t, e, targetKind, newBlockingTargetHandler)

	handler := &blockingTargetHandler{
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	targetCfg := testTargetConfig(targetKind, map[string]any{"handler": handler})

	backend, err := e.NewTarget(targetCfg)
	if err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	closeDone := make(chan struct{})
	go func() {
		backend.Close()
		close(closeDone)
	}()
	waitForSignal(t, closeDone, "target Close() to return")
	waitForSignal(t, handler.closeStarted, "target handler Close() to start")

	if backend.Warm() {
		t.Fatal("Warm() after Close() = true, want false")
	}
	if backend.Drain() {
		t.Fatal("Drain() after Close() = true, want false")
	}

	waitDone := make(chan struct{})
	go func() {
		backend.Wait()
		close(waitDone)
	}()

	assertNoSignal(t, waitDone, "target Wait() returned while handler Close() was still blocked")

	handler.Release()
	waitForSignal(t, waitDone, "target Wait() after releasing handler Close()")
}

func TestTargetCloseDuringWarmKeepsClosed(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("blocking-target")
	mustAddTargetKind(t, e, targetKind, newBlockingTargetHandler)

	handler := &blockingTargetHandler{
		warmStarted:  make(chan struct{}),
		releaseWarm:  make(chan struct{}),
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	backend := mustNewBlockingTarget(t, e, targetKind, handler)

	if !backend.Warm() {
		t.Fatal("Warm() = false, want true")
	}
	waitForSignal(t, handler.warmStarted, "target handler Warm() to start")

	backend.Close()
	if backend.State() != target.Closed {
		t.Fatalf("State() after Close() = %s, want closed", backend.State())
	}

	handler.ReleaseWarm()
	waitForSignal(t, handler.closeStarted, "target handler Close() to start")
	handler.ReleaseClose()
	backend.Wait()

	if backend.State() != target.Closed {
		t.Fatalf("State() after Wait() = %s, want closed", backend.State())
	}
}

func TestTargetCloseDuringDrainKeepsClosed(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("blocking-target")
	mustAddTargetKind(t, e, targetKind, newBlockingTargetHandler)

	handler := &blockingTargetHandler{
		drainStarted: make(chan struct{}),
		releaseDrain: make(chan struct{}),
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	backend := mustNewBlockingTarget(t, e, targetKind, handler)

	if !backend.Warm() {
		t.Fatal("Warm() = false, want true")
	}
	waitForTargetState(t, backend, target.Active)

	if !backend.Drain() {
		t.Fatal("Drain() = false, want true")
	}
	waitForSignal(t, handler.drainStarted, "target handler Drain() to start")

	backend.Close()
	if backend.State() != target.Closed {
		t.Fatalf("State() after Close() = %s, want closed", backend.State())
	}

	handler.ReleaseDrain()
	waitForSignal(t, handler.closeStarted, "target handler Close() to start")
	handler.ReleaseClose()
	backend.Wait()

	if backend.State() != target.Closed {
		t.Fatalf("State() after Wait() = %s, want closed", backend.State())
	}
}

func TestFrontendCloseDoesNotWaitForHandlerClose(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("target")
	frontendKind := testKind("blocking-frontend")
	mustAddTargetKind(t, e, targetKind, newStubTargetHandler)
	mustAddFrontendKind(t, e, frontendKind, newBlockingFrontendHandler)

	targetCfg := testTargetConfig(targetKind, nil)
	if _, err := e.NewTarget(targetCfg); err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	handler := &blockingFrontendHandler{
		shouldWarm:   make(chan struct{}),
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	frontendCfg := testFrontendConfig(frontendKind, map[string]any{"handler": handler})

	listener, err := e.NewFrontend(frontendCfg)
	if err != nil {
		t.Fatalf("NewFrontend() error = %v", err)
	}

	closeDone := make(chan struct{})
	go func() {
		listener.Close()
		close(closeDone)
	}()
	waitForSignal(t, closeDone, "frontend Close() to return")
	waitForSignal(t, handler.closeStarted, "frontend handler Close() to start")

	if listener.Start() {
		t.Fatal("Start() after Close() = true, want false")
	}
	if listener.Stop() {
		t.Fatal("Stop() after Close() = true, want false")
	}

	waitDone := make(chan struct{})
	go func() {
		listener.Wait()
		close(waitDone)
	}()

	assertNoSignal(t, waitDone, "frontend Wait() returned while handler Close() was still blocked")

	handler.Release()
	waitForSignal(t, waitDone, "frontend Wait() after releasing handler Close()")
}

func TestFrontendCloseDuringStartKeepsClosed(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("target")
	frontendKind := testKind("blocking-frontend")
	mustAddTargetKind(t, e, targetKind, newStubTargetHandler)
	mustAddFrontendKind(t, e, frontendKind, newBlockingFrontendHandler)

	handler := &blockingFrontendHandler{
		shouldWarm:   make(chan struct{}),
		startStarted: make(chan struct{}),
		releaseStart: make(chan struct{}),
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	listener := mustNewBlockingFrontend(t, e, targetKind, frontendKind, handler)

	if !listener.Start() {
		t.Fatal("Start() = false, want true")
	}
	waitForSignal(t, handler.startStarted, "frontend handler Start() to start")

	listener.Close()
	if listener.State() != frontend.Closed {
		t.Fatalf("State() after Close() = %s, want closed", listener.State())
	}

	handler.ReleaseStart()
	waitForSignal(t, handler.closeStarted, "frontend handler Close() to start")
	handler.ReleaseClose()
	listener.Wait()

	if listener.State() != frontend.Closed {
		t.Fatalf("State() after Wait() = %s, want closed", listener.State())
	}
}

func TestFrontendCloseDuringStopKeepsClosed(t *testing.T) {
	e := mustNewTestEngine(t)

	targetKind := testKind("target")
	frontendKind := testKind("blocking-frontend")
	mustAddTargetKind(t, e, targetKind, newStubTargetHandler)
	mustAddFrontendKind(t, e, frontendKind, newBlockingFrontendHandler)

	handler := &blockingFrontendHandler{
		shouldWarm:   make(chan struct{}),
		stopStarted:  make(chan struct{}),
		releaseStop:  make(chan struct{}),
		closeStarted: make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	t.Cleanup(handler.Release)
	listener := mustNewBlockingFrontend(t, e, targetKind, frontendKind, handler)

	if !listener.Start() {
		t.Fatal("Start() = false, want true")
	}
	waitForFrontendState(t, listener, frontend.Running)

	if !listener.Stop() {
		t.Fatal("Stop() = false, want true")
	}
	waitForSignal(t, handler.stopStarted, "frontend handler Stop() to start")

	listener.Close()
	if listener.State() != frontend.Closed {
		t.Fatalf("State() after Close() = %s, want closed", listener.State())
	}

	handler.ReleaseStop()
	waitForSignal(t, handler.closeStarted, "frontend handler Close() to start")
	handler.ReleaseClose()
	listener.Wait()

	if listener.State() != frontend.Closed {
		t.Fatalf("State() after Wait() = %s, want closed", listener.State())
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

func testKind(name string) config.NamespaceReference {
	return config.NamespaceReference{
		Namespace: "test",
		Name:      name,
	}
}

func mustAddTargetKind(t *testing.T, e *Engine, kind config.NamespaceReference, ctor target.HandlerCtor) {
	t.Helper()

	if err := e.AddTargetKind(kind.String(), ctor); err != nil {
		t.Fatalf("AddTargetKind() error = %v", err)
	}
}

func mustAddFrontendKind(t *testing.T, e *Engine, kind config.NamespaceReference, ctor frontend.HandlerCtor) {
	t.Helper()

	if err := e.AddFrontendKind(kind.String(), ctor); err != nil {
		t.Fatalf("AddFrontendKind() error = %v", err)
	}
}

func testTargetConfig(kind config.NamespaceReference, options map[string]any) config.Target {
	return config.Target{
		Name:        "backend",
		Kind:        kind,
		IdleTimeout: config.TTL(30),
		Options:     options,
		Endpoints: []config.TargetEndpoint{
			{
				Name:     "http",
				Protocol: config.ProtocolTCP,
				Address:  netip.MustParseAddrPort("127.0.0.2:8080"),
			},
		},
	}
}

func testFrontendConfig(kind config.NamespaceReference, options map[string]any) config.Frontend {
	return config.Frontend{
		Name:        "listener",
		Kind:        kind,
		Protocol:    config.ProtocolTCP,
		Listen:      netip.MustParseAddrPort("127.0.0.1:8080"),
		FlowTimeout: config.TTL(15),
		Endpoint: config.NamespaceReference{
			Namespace: "backend",
			Name:      "http",
		},
		Options: options,
	}
}

func mustNewBlockingTarget(t *testing.T, e *Engine, kind config.NamespaceReference, handler *blockingTargetHandler) *target.Target {
	t.Helper()

	targetCfg := testTargetConfig(kind, map[string]any{"handler": handler})
	got, err := e.NewTarget(targetCfg)
	if err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}
	return got
}

func mustNewBlockingFrontend(t *testing.T, e *Engine, targetKind, frontendKind config.NamespaceReference, handler *blockingFrontendHandler) *frontend.Frontend {
	t.Helper()

	targetCfg := testTargetConfig(targetKind, nil)
	if _, err := e.NewTarget(targetCfg); err != nil {
		t.Fatalf("NewTarget() error = %v", err)
	}

	frontendCfg := testFrontendConfig(frontendKind, map[string]any{"handler": handler})
	got, err := e.NewFrontend(frontendCfg)
	if err != nil {
		t.Fatalf("NewFrontend() error = %v", err)
	}
	return got
}

func waitForTargetState(t *testing.T, got *target.Target, want target.State) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("State() = %s, want %s", got.State(), want)
}

func waitForFrontendState(t *testing.T, got *frontend.Frontend, want frontend.State) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("State() = %s, want %s", got.State(), want)
}

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertNoSignal(t *testing.T, ch <-chan struct{}, failure string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal(failure)
	case <-time.After(20 * time.Millisecond):
	}
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

type blockingTargetHandler struct {
	warmStarted     chan struct{}
	releaseWarm     chan struct{}
	warmOnce        sync.Once
	releaseWarmOnce sync.Once

	drainStarted     chan struct{}
	releaseDrain     chan struct{}
	drainOnce        sync.Once
	releaseDrainOnce sync.Once

	closeStarted     chan struct{}
	releaseClose     chan struct{}
	closeOnce        sync.Once
	releaseCloseOnce sync.Once
}

func newBlockingTargetHandler(name string, options map[string]any) (target.Handler, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	handler, ok := options["handler"].(*blockingTargetHandler)
	if !ok {
		return nil, fmt.Errorf("handler is required")
	}
	return handler, nil
}

func (h *blockingTargetHandler) Warm() error {
	if h.warmStarted == nil {
		return nil
	}
	h.warmOnce.Do(func() {
		close(h.warmStarted)
	})
	if h.releaseWarm != nil {
		<-h.releaseWarm
	}
	return nil
}

func (h *blockingTargetHandler) ReleaseWarm() {
	closeOnce(h.releaseWarm, &h.releaseWarmOnce)
}

func (h *blockingTargetHandler) Drain() error {
	if h.drainStarted == nil {
		return nil
	}
	h.drainOnce.Do(func() {
		close(h.drainStarted)
	})
	if h.releaseDrain != nil {
		<-h.releaseDrain
	}
	return nil
}

func (h *blockingTargetHandler) ReleaseDrain() {
	closeOnce(h.releaseDrain, &h.releaseDrainOnce)
}

func (h *blockingTargetHandler) Close() error {
	closeOnce(h.closeStarted, &h.closeOnce)
	if h.releaseClose != nil {
		<-h.releaseClose
	}
	return nil
}

func (h *blockingTargetHandler) ReleaseClose() {
	closeOnce(h.releaseClose, &h.releaseCloseOnce)
}

func (h *blockingTargetHandler) Release() {
	h.ReleaseWarm()
	h.ReleaseDrain()
	h.ReleaseClose()
}

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

type blockingFrontendHandler struct {
	shouldWarm       chan struct{}
	startStarted     chan struct{}
	releaseStart     chan struct{}
	startOnce        sync.Once
	releaseStartOnce sync.Once

	stopStarted     chan struct{}
	releaseStop     chan struct{}
	stopOnce        sync.Once
	releaseStopOnce sync.Once

	closeStarted     chan struct{}
	releaseClose     chan struct{}
	closeOnce        sync.Once
	releaseCloseOnce sync.Once
}

func newBlockingFrontendHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	handler, ok := options["handler"].(*blockingFrontendHandler)
	if !ok {
		return nil, fmt.Errorf("handler is required")
	}
	return handler, nil
}

func (h *blockingFrontendHandler) Start() error {
	if h.startStarted == nil {
		return nil
	}
	h.startOnce.Do(func() {
		close(h.startStarted)
	})
	if h.releaseStart != nil {
		<-h.releaseStart
	}
	return nil
}

func (h *blockingFrontendHandler) ReleaseStart() {
	closeOnce(h.releaseStart, &h.releaseStartOnce)
}

func (h *blockingFrontendHandler) Stop() error {
	if h.stopStarted == nil {
		return nil
	}
	h.stopOnce.Do(func() {
		close(h.stopStarted)
	})
	if h.releaseStop != nil {
		<-h.releaseStop
	}
	return nil
}

func (h *blockingFrontendHandler) ReleaseStop() {
	closeOnce(h.releaseStop, &h.releaseStopOnce)
}

func (h *blockingFrontendHandler) Close() error {
	closeOnce(h.closeStarted, &h.closeOnce)
	if h.releaseClose != nil {
		<-h.releaseClose
	}
	return nil
}

func (h *blockingFrontendHandler) ReleaseClose() {
	closeOnce(h.releaseClose, &h.releaseCloseOnce)
}

func (h *blockingFrontendHandler) Release() {
	h.ReleaseStart()
	h.ReleaseStop()
	h.ReleaseClose()
}

func (h *blockingFrontendHandler) ShouldWarm() <-chan struct{} { return h.shouldWarm }

func closeOnce(ch chan struct{}, once *sync.Once) {
	if ch == nil {
		return
	}
	once.Do(func() {
		close(ch)
	})
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
