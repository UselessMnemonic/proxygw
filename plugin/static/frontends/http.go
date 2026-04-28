package frontends

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"

	"proxygw/pkg/config"
	"proxygw/pkg/frontend"
)

const httpShutdownTimeout = 5 * time.Second

type HTTPHandler struct {
	address  netip.AddrPort
	content  string
	endpoint string
	warm     chan struct{}

	logger *slog.Logger
	server *http.Server
}

func (h *HTTPHandler) Start() error {
	if h.server != nil {
		h.logger.Info("http frontend already started", "listen", h.address.String())
		return nil
	}

	h.logger.Info("starting http frontend", "listen", h.address.String())
	listener, err := net.Listen("tcp", h.address.String())
	if err != nil {
		h.logger.Error("http frontend listen failed", "listen", h.address.String(), "err", err)
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(h.endpoint, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(h.content))
		h.logger.Debug("http request served", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr, "warm_queued", h.tickWarm())
	})

	server := &http.Server{
		Handler: mux,
	}
	h.server = server

	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error("http frontend serve failed", "err", err)
			_ = server.Close()
		}
	}()

	h.logger.Info("http frontend started", "listen", h.address.String())
	return nil
}

func (h *HTTPHandler) Stop() error {
	if h.server == nil {
		h.logger.Info("http frontend already stopped")
		return nil
	}

	h.logger.Info("stopping http frontend")
	ctx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()

	err := h.server.Shutdown(ctx)
	h.server = nil
	if err != nil {
		h.logger.Info("http frontend shutdown completed with error", "err", err)
	}
	h.logger.Info("http frontend stopped")
	return nil
}

func (h *HTTPHandler) Close() error {
	return h.Stop()
}

func (h *HTTPHandler) ShouldWarm() <-chan struct{} {
	return h.warm
}

func (h *HTTPHandler) tickWarm() bool {
	select {
	case h.warm <- struct{}{}:
		return true
	default:
		return false
	}
}

func NewHTTPHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	if protocol != config.ProtocolTCP {
		return nil, fmt.Errorf("static http frontend requires tcp protocol")
	}

	content, ok := options["content"].(string)
	if !ok {
		return nil, fmt.Errorf("static http frontend option content is required")
	}

	endpoint, err := httpEndpointOption(options)
	if err != nil {
		return nil, err
	}

	return &HTTPHandler{
		address:  address,
		content:  content,
		endpoint: endpoint,
		logger:   slog.Default().With("component", "static:http", "name", name, "endpoint", endpoint),
		warm:     make(chan struct{}, 1),
	}, nil
}

func httpEndpointOption(options map[string]any) (string, error) {
	endpoint, ok := options["endpoint"].(string)
	if !ok {
		return "/", nil
	}
	if endpoint == "" || endpoint[0] != '/' {
		return "", fmt.Errorf("static http frontend option endpoint must start with /")
	}
	return endpoint, nil
}
