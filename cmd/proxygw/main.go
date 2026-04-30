package main

//go:generate go run ../tool/gen.go

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"

	"github.com/UselessMnemonic/proxygw/internal/plugin"
	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane"
	"github.com/UselessMnemonic/proxygw/pkg/dataplane/connft"
	"github.com/UselessMnemonic/proxygw/pkg/engine"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

func setDefaultLogger(dst io.Writer, level slog.Level) *slog.Logger {
	options := slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(dst, &options)
	logger := slog.New(handler).With("component", "proxygw")
	slog.SetDefault(logger)
	return logger
}

func main() {
	logger := setDefaultLogger(os.Stderr, slog.LevelInfo)

	app := kingpin.New("proxygw", "Proxy gateway daemon")

	configPath := app.Flag("config", "path to runtime configuration").
		Default("/etc/proxygw.yaml").
		String()

	if _, err := app.Parse(os.Args[1:]); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	err := run(logger, *configPath)
	logger = slog.Default()

	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger, configPath string) (err error) {
	logger.Info("loading config", "path", configPath)
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config %q: %w", configPath, err)
	}

	var logout io.Writer
	switch cfg.Log.Output {
	case "stdout":
		logout = os.Stdout
	case "stderr":
		logout = os.Stderr
	default:
		logout, err = os.OpenFile(cfg.Log.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log output %q: %w", cfg.Log.Output, err)
		}
	}
	logger = setDefaultLogger(logout, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("creating dataplane", "name", "proxygw")
	dplane, err := connft.New("proxygw")
	if err != nil {
		return fmt.Errorf("create dataplane: %w", err)
	}
	logger.Info("dataplane created", "name", "proxygw")

	logger.Info("creating engine", "name", "proxygw")
	eng, err := engine.New(ctx, dplane)
	if err != nil {
		closeDataplane(logger, dplane)
		return fmt.Errorf("create engine: %w", err)
	}
	logger.Info("engine created", "name", "proxygw")

	err = pluginScope(logger, ctx, cfg, eng)
	closeEngine(logger, eng)
	closeDataplane(logger, dplane)
	return err
}

func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func pluginScope(logger *slog.Logger, ctx context.Context, cfg *config.Config, eng *engine.Engine) (err error) {
	registry := plugin.Export()
	for pluginName, handler := range registry {
		namespace := &plugin.Namespace{
			Frontends: make(map[string]frontend.HandlerCtor),
			Targets:   make(map[string]target.HandlerCtor),
		}

		var pluginConfig map[string]any
		if cfg.Plugins != nil {
			pluginConfig = cfg.Plugins[pluginName]
		}

		if handler.OnLoad != nil {
			logger.Info("loading plugin", "plugin", pluginName)
			if err := handler.OnLoad(pluginConfig, eng, namespace); err != nil {
				return fmt.Errorf("load plugin %q: %w", pluginName, err)
			}
			logger.Info("plugin loaded", "plugin", pluginName)
			defer func(pluginName string, handler plugin.Handler) {
				if handler.OnUnload == nil {
					return
				}
				err = errors.Join(err, unloadPlugin(logger, pluginName, handler))
			}(pluginName, handler)
		}

		if err := registerKinds(eng, pluginName, namespace); err != nil {
			return fmt.Errorf("register plugin %q kinds: %w", pluginName, err)
		}
		logger.Info("plugin kinds registered", "plugin", pluginName, "frontends", len(namespace.Frontends), "targets", len(namespace.Targets))
	}

	return resourceScope(logger, ctx, cfg, eng)
}

func resourceScope(logger *slog.Logger, ctx context.Context, cfg *config.Config, eng *engine.Engine) error {
	logger.Info("applying config", "targets", len(cfg.Targets), "frontends", len(cfg.Frontends))
	if err := applyConfig(logger, cfg, eng); err != nil {
		return err
	}
	logger.Info("config applied", "targets", len(cfg.Targets), "frontends", len(cfg.Frontends))

	logger.Info("waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("shutdown signal received", "err", ctx.Err())
	return nil
}

func registerKinds(eng *engine.Engine, pluginName string, namespace *plugin.Namespace) error {
	for exportedKind := range namespace.Frontends {
		kindRef := config.NamespaceReference{
			Namespace: pluginName,
			Name:      exportedKind,
		}
		if err := eng.AddFrontendKind(kindRef.String(), namespace.Frontends[exportedKind]); err != nil {
			return fmt.Errorf("frontend kind %q: %w", kindRef.String(), err)
		}
	}

	for exportedKind := range namespace.Targets {
		kindRef := config.NamespaceReference{
			Namespace: pluginName,
			Name:      exportedKind,
		}
		if err := eng.AddTargetKind(kindRef.String(), namespace.Targets[exportedKind]); err != nil {
			return fmt.Errorf("target kind %q: %w", kindRef.String(), err)
		}
	}

	return nil
}

func applyConfig(logger *slog.Logger, cfg *config.Config, eng *engine.Engine) error {
	for _, targetCfg := range cfg.Targets {
		logger.Info("creating target", "name", targetCfg.Name, "kind", targetCfg.Kind.String())
		if _, err := eng.NewTarget(targetCfg); err != nil {
			return fmt.Errorf("create target %q: %w", targetCfg.Name, err)
		}
		logger.Info("target created", "name", targetCfg.Name, "kind", targetCfg.Kind.String())
	}

	for _, frontendCfg := range cfg.Frontends {
		logger.Info("creating frontend", "name", frontendCfg.Name, "kind", frontendCfg.Kind.String(), "listen", frontendCfg.Listen.String())
		frontend, err := eng.NewFrontend(frontendCfg)
		if err != nil {
			logger.Error("create frontend failed", "name", frontendCfg.Name, "kind", frontendCfg.Kind.String(), "err", err)
			continue
		}
		logger.Info("frontend created", "name", frontendCfg.Name, "kind", frontendCfg.Kind.String(), "listen", frontendCfg.Listen.String())
		if !frontend.Start() {
			logger.Error("start frontend rejected", "name", frontendCfg.Name, "kind", frontendCfg.Kind.String(), "err", errors.New("start rejected"))
			continue
		}
		logger.Info("frontend start requested", "name", frontendCfg.Name, "kind", frontendCfg.Kind.String())
	}

	return nil
}

func unloadPlugin(logger *slog.Logger, name string, handler plugin.Handler) error {
	if handler.OnUnload == nil {
		return nil
	}
	logger.Info("unloading plugin", "plugin", name)
	if err := handler.OnUnload(); err != nil {
		return fmt.Errorf("unload plugin %q: %w", name, err)
	}
	logger.Info("plugin unloaded", "plugin", name)
	return nil
}

func closeEngine(logger *slog.Logger, eng *engine.Engine) {
	if eng == nil {
		return
	}
	logger.Info("closing engine")
	eng.Close()
	eng.Wait()
	logger.Info("engine closed")
}

func closeDataplane(logger *slog.Logger, dplane dataplane.Dataplane) {
	if dplane == nil {
		return
	}
	logger.Info("closing dataplane", "name", dplane.Name())
	if err := dplane.Close(); err != nil {
		logger.Error("close dataplane failed", "name", dplane.Name(), "err", err)
		return
	}
	logger.Info("dataplane closed", "name", dplane.Name())
}
