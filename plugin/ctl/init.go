package ctl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"proxygw/internal/engine"
	"proxygw/pkg/ipc"
	"proxygw/pkg/ipc/codec"
	"proxygw/plugin"
)

var ctx context.Context
var cancel context.CancelFunc

func listenLoop(listener *ipc.Listener, engine *engine.Engine) {
	logger := log.New(os.Stderr, "ctl: ", log.LstdFlags)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return
			}
			logger.Println("accept error:", err)
		}
		server := NewServer(conn, engine)
		go server.Serve(ctx)
	}
}

func onLoad(config map[string]any, engine *engine.Engine, _ *plugin.Namespace) error {
	socket, ok := config["socket"].(string)
	if !ok {
		return errors.New("missing socket path")
	}

	c, ok := config["codec"].(string)
	if !ok {
		c = "json"
	}
	codec := codec.FindByName(c)
	if codec == nil {
		return fmt.Errorf("invalid codec: %q", c)
	}

	listener, err := ipc.Listen("unix", socket, codec)
	if err != nil {
		return err
	}

	go listenLoop(listener, engine)
	return nil
}

func onUnload() error {
	cancel()
	return nil
}

func init() {
	err := plugin.Register("ctl", plugin.Handler{
		OnLoad:   onLoad,
		OnUnload: onUnload,
	})
	if err != nil {
		panic(err)
	}
}
