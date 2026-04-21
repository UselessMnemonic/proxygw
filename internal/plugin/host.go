package plugin

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"proxygw/internal/frontend"
	"proxygw/internal/target"
	"proxygw/pkg/config"
	"proxygw/pkg/ipc"
	"proxygw/pkg/ipc/codec"
	"proxygw/pkg/ipc/method"
	"slices"
	"sync"
	"syscall"
	"time"
)

const defaultTimeout = 5 * time.Second

type Host struct {
	m      sync.RWMutex
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	cmd   *exec.Cmd
	codec codec.Codec
	err   error

	frontendKinds   map[string]frontend.Kind
	frontendDrivers map[string]pluginFrontendDriver
	targetKinds     map[string]target.Kind
	targetDrivers   map[string]pluginTargetDriver

	listener   *ipc.Listener
	client     *ipc.BaseClient
	tempDir    string
	socketPath string
}

func NewHost(ctx context.Context, manifest config.PluginManifest) (*Host, error) {
	codec := codec.FindByName(manifest.Codec)
	if codec == nil {
		return nil, fmt.Errorf("no such codec %s", manifest.Codec)
	}

	tempDir, err := os.MkdirTemp("", "proxygw-plugin-*")
	if err != nil {
		return nil, fmt.Errorf("create plugin socket: %w", err)
	}

	socketPath := filepath.Join(tempDir, "plugin.sock")
	listener, err := ipc.Listen("unix", socketPath, codec)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("create plugin socket: %w", err)
	}

	cmd := exec.Command(manifest.Cmd, manifest.Args...)
	cmd.WaitDelay = defaultTimeout
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOCKET_PATH=%s", socketPath))
	err = cmd.Start()
	if err != nil {
		listener.Close()
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("start plugin process: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	host := &Host{
		m:      sync.RWMutex{},
		wg:     sync.WaitGroup{},
		ctx:    ctx,
		cancel: cancel,

		cmd:   cmd,
		codec: codec,
		err:   nil,

		frontendKinds:   make(map[string]frontend.Kind),
		frontendDrivers: make(map[string]pluginFrontendDriver),
		targetKinds:     make(map[string]target.Kind),
		targetDrivers:   make(map[string]pluginTargetDriver),

		listener:   listener,
		client:     nil,
		tempDir:    tempDir,
		socketPath: socketPath,
	}
	for _, f := range manifest.Frontends {
		host.frontendKinds[f] = nil
	}
	for _, t := range manifest.Targets {
		host.targetKinds[t] = nil
	}

	return host, nil
}

func (h *Host) Cmd() string {
	return h.cmd.String()
}

func (h *Host) Codec() codec.Codec {
	return h.codec
}

func (h *Host) FrontendKinds() []frontend.Kind {
	h.m.RLock()
	defer h.m.RUnlock()
	return slices.Collect(maps.Values(h.frontendKinds))
}

func (h *Host) TargetKinds() []target.Kind {
	h.m.RLock()
	defer h.m.RUnlock()
	return slices.Collect(maps.Values(h.targetKinds))
}

func (h *Host) Error() error {
	h.m.RLock()
	defer h.m.RUnlock()
	return h.err
}

func (h *Host) Close() {
	h.cancel()
}

func (h *Host) Wait() {
	<-h.ctx.Done()
	h.wg.Wait()
}

func (h *Host) Start() error {
	// wait for someone to connect to the socket
	conn, err := h.waitForAccept()
	if err != nil {
		h.setErr(err)
		h.end()
		return err
	}
	h.client = ipc.NewBaseClient(conn)
	h.loop()

	// init the plugin
	err = h.tryInit()
	if err != nil {
		h.setErr(err)
		h.Close()
		return err
	}

	return nil
}

func (h *Host) end() {
	h.Close()
	h.cmd.Process.Signal(syscall.SIGTERM) // blindly ask process to close
	h.cmd.Wait()                          // process will have closed here or be killed after the wait timeout
	h.listener.Close()
	h.client.Close()
}

func (h *Host) waitForAccept() (*ipc.Conn, error) {
	type acceptResult struct {
		*ipc.Conn
		error
	}

	connCh := make(chan acceptResult)
	go func() {
		conn, err := h.listener.Accept()
		connCh <- acceptResult{conn, err}
		close(connCh)
		h.listener.Close()
	}()

	timer := time.NewTimer(defaultTimeout)
	select {
	case <-h.ctx.Done():
		return nil, context.Canceled
	case <-timer.C:
		return nil, context.DeadlineExceeded
	case result := <-connCh:
		return result.Conn, result.error
	}
}

func (h *Host) waitForResponse(packet *ipc.Packet) error {
	res, err := h.client.Request(packet)
	if err != nil {
		return err
	}

	timer := time.NewTimer(defaultTimeout)
	select {
	case <-h.ctx.Done():
		return context.Canceled
	case <-timer.C:
		return context.DeadlineExceeded
	case res, ok := <-res:
		if !ok {
			return context.Canceled
		}
		*packet = res
		return nil
	}
}

func (h *Host) setErr(err error) {
	h.m.Lock()
	defer h.m.Unlock()
	h.err = err
}

func (h *Host) tryInit() error {
	packet := ipc.Packet{
		Method: method.MethodPluginInitRequest,
		Body:   method.PluginInitRequest{},
	}
	if err := h.waitForResponse(&packet); err != nil {
		return err
	}

	switch packet.Method {
	case method.MethodPluginInitResponse:
		result := packet.Body.(*method.PluginInitResponse)
		h.m.Lock()
		defer h.m.Unlock()

		if slices.Compare(
			slices.Compact(slices.Sorted(slices.Values(result.FrontendKinds))),
			slices.Sorted(maps.Keys(h.frontendKinds)),
		) != 0 {
			return fmt.Errorf("frontend mismatch: expected %s, got %s", h.frontendKinds, result.FrontendKinds)
		}

		if slices.Compare(
			slices.Compact(slices.Sorted(slices.Values(result.TargetKinds))),
			slices.Sorted(maps.Keys(h.targetKinds)),
		) != 0 {
			return fmt.Errorf("target mismatch: expected %s, got %s", h.targetKinds, result.TargetKinds)
		}

		for name, _ := range h.frontendKinds {
			h.frontendKinds[name] = h.newFrontendKind(name)
		}
		for name, _ := range h.targetKinds {
			h.targetKinds[name] = h.newTargetKind(name)
		}

		return nil
	case method.MethodErrorResponse:
		return packet.Body.(*method.ErrorResponse)
	default:
		return fmt.Errorf("unexpected response %v", packet.Body)
	}
}

func (h *Host) tryShouldWarm(name string) {
	h.m.RLock()
	defer h.m.RUnlock()

	if driver, ok := h.frontendDrivers[name]; ok {
		driver.shouldWarm <- struct{}{}
	}
}

func (h *Host) loop() {
	h.wg.Go(func() {
		defer h.end()
		for {
			select {
			case <-h.ctx.Done():
				return
			case notif, ok := <-h.client.Notifications():
				if !ok {
					h.setErr(context.Canceled)
					return
				}
				switch notif.Method {
				case method.MethodFrontendShouldWarmNotification:
					notif := notif.Body.(*method.FrontendShouldWarmNotification)
					h.tryShouldWarm(notif.Name)
				default:
					continue
				}
			}
		}
	})
}
