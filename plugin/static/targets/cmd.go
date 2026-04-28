package targets

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"proxygw/pkg/target"
)

const cmdStopTimeout = 5 * time.Second

// CmdHandler is a target driver that runs a shell command while the target is
// warm.
type CmdHandler struct {
	command string
	logger  *slog.Logger

	cmd  *exec.Cmd
	done chan error
}

// Warm starts the configured command if it is not already running.
func (h *CmdHandler) Warm() error {
	if h.cmd != nil {
		h.logger.Info("command already started")
		return nil
	}

	h.logger.Info("starting command", "command", h.command)
	cmd := commandLine(h.command)
	if err := cmd.Start(); err != nil {
		h.logger.Error("command start failed", "command", h.command, "err", err)
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	h.cmd = cmd
	h.done = done
	h.logger.Info("command started", "command", h.command, "pid", cmd.Process.Pid)
	return nil
}

// Drain interrupts the configured command and waits briefly before killing it.
func (h *CmdHandler) Drain() error {
	cmd := h.cmd
	done := h.done
	if cmd == nil {
		h.logger.Info("command already stopped")
		return nil
	}
	h.cmd = nil
	h.done = nil

	if cmd.Process == nil {
		h.logger.Info("command process missing")
		return nil
	}
	h.logger.Info("stopping command", "command", h.command, "pid", cmd.Process.Pid)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		h.logger.Info("command interrupt failed; killing process", "command", h.command, "pid", cmd.Process.Pid, "err", err)
		_ = cmd.Process.Kill()
	}

	select {
	case err := <-done:
		if err != nil {
			h.logger.Info("command stopped with error", "command", h.command, "pid", cmd.Process.Pid, "err", err)
		} else {
			h.logger.Info("command stopped", "command", h.command, "pid", cmd.Process.Pid)
		}
		return err
	case <-time.After(cmdStopTimeout):
		h.logger.Info("command stop timed out; killing process", "command", h.command, "pid", cmd.Process.Pid)
		if err := cmd.Process.Kill(); err != nil {
			h.logger.Error("command kill failed", "command", h.command, "pid", cmd.Process.Pid, "err", err)
			return err
		}
		err := <-done
		if err != nil {
			h.logger.Info("command killed with error", "command", h.command, "pid", cmd.Process.Pid, "err", err)
		} else {
			h.logger.Info("command killed", "command", h.command, "pid", cmd.Process.Pid)
		}
		return err
	}
}

// Close stops the command if it is still running.
func (h *CmdHandler) Close() error {
	return h.Drain()
}

// NewCmdHandler creates a cmd target from options. The "command" option is
// required.
func NewCmdHandler(name string, options map[string]any) (target.Handler, error) {
	command, ok := options["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("static cmd target option command is required")
	}

	return &CmdHandler{
		command: command,
		logger:  slog.Default().With("component", "static:cmd", "name", name),
	}, nil
}

func commandLine(command string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", command)
}
