package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"proxygw/pkg/config"
	"proxygw/pkg/ipc"
	"proxygw/pkg/ipc/codec"
	"time"
)

const (
	defaultDialTimeout = 10 * time.Second
)

type Host struct {
	cmd        *exec.Cmd
	tempDir    string
	socketPath string

	codec    codec.Codec
	listener *ipc.Listener
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

	cmd := exec.CommandContext(ctx, manifest.Cmd, manifest.Args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOCKET_PATH=%s", socketPath))

	host := &Host{
		cmd:        cmd,
		tempDir:    tempDir,
		socketPath: socketPath,

		codec: codec,
	}

	go func() {
		<-ctx.Done()
		_ = host.Close()
	}()

	return host, nil
}

func (h *Host) Close() error {
	var err error
	if h.listener != nil {
		err = h.listener.Close()
	}
	if removeErr := os.RemoveAll(h.socketDir); err == nil {
		err = removeErr
	}
	return err
}
