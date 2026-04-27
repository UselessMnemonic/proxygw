package main

//go:generate go run -mod=readonly ../tool/gen.go

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"

	"proxygw/internal/plugin"
	"proxygw/pkg/config"
	"proxygw/pkg/engine"
	"proxygw/pkg/frontend"
	"proxygw/pkg/target"
)

func main() {
	app := kingpin.New("proxygw", "Proxy gateway daemon")

	configPath := app.Flag("config", "path to runtime configuration").
		Default("/etc/proxygw.yaml").
		String()

	if _, err := app.Parse(os.Args[1:]); err != nil {
		log.Printf("proxygw: %v", err)
		os.Exit(1)
	}

	err := run(*configPath)
	if err != nil {
		log.Printf("proxygw: %v", err)
		os.Exit(1)
	}
}

func run(configPath string) (err error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config %q: %w", configPath, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	eng, err := engine.New(ctx, "proxygw")
	if err != nil {
		return fmt.Errorf("create engine: %w", err)
	}
	defer closeEngine(eng)

	return pluginScope(ctx, cfg, eng)
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

func pluginScope(ctx context.Context, cfg *config.Config, eng *engine.Engine) (err error) {
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
			if err := handler.OnLoad(pluginConfig, eng, namespace); err != nil {
				return fmt.Errorf("load plugin %q: %w", pluginName, err)
			}
			defer func(pluginName string, handler plugin.Handler) {
				if handler.OnUnload == nil {
					return
				}
				err = errors.Join(err, unloadPlugin(pluginName, handler))
			}(pluginName, handler)
		}

		if err := registerKinds(eng, pluginName, namespace); err != nil {
			return fmt.Errorf("register plugin %q kinds: %w", pluginName, err)
		}
	}

	return resourceScope(ctx, cfg, eng)
}

func resourceScope(ctx context.Context, cfg *config.Config, eng *engine.Engine) error {
	if err := applyConfig(cfg, eng); err != nil {
		return err
	}

	<-ctx.Done()
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

func applyConfig(cfg *config.Config, eng *engine.Engine) error {
	for _, targetCfg := range cfg.Targets {
		if _, err := eng.NewTarget(targetCfg); err != nil {
			return fmt.Errorf("create target %q: %w", targetCfg.Name, err)
		}
	}

	for _, frontendCfg := range cfg.Frontends {
		if _, err := eng.NewFrontend(frontendCfg); err != nil {
			return fmt.Errorf("create frontend %q: %w", frontendCfg.Name, err)
		}
	}

	return nil
}

func unloadPlugin(name string, handler plugin.Handler) error {
	if handler.OnUnload == nil {
		return nil
	}
	if err := handler.OnUnload(); err != nil {
		return fmt.Errorf("unload plugin %q: %w", name, err)
	}
	return nil
}

func closeEngine(eng *engine.Engine) {
	if eng == nil {
		return
	}
	eng.Close()
	eng.Wait()
}
