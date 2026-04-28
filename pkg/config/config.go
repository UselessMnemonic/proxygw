package config

import (
	"errors"
	"fmt"
	"log/slog"
)

// ConfigVersionV1 is the supported schema version for configuration files.
const ConfigVersionV1 = "v1"

// Config is the root runtime configuration document.
type Config struct {
	Version   string                    `yaml:"version"`
	Log       Log                       `yaml:"log"`
	Plugins   map[string]map[string]any `yaml:"plugins"`
	Targets   []Target                  `yaml:"targets"`
	Frontends []Frontend                `yaml:"frontends"`
}

// Log defines process-wide structured logging behavior.
type Log struct {
	Output string     `yaml:"output"`
	Level  slog.Level `yaml:"level"`
}

// Validate checks that the configuration is internally consistent and usable.
func (c *Config) Validate() error {
	if c.Version == "" {
		return errors.New("version is required")
	}
	if c.Version != ConfigVersionV1 {
		return fmt.Errorf("unsupported version %q", c.Version)
	}
	if c.Log.Output == "" {
		return errors.New("log.output is required")
	}

	for i, target := range c.Targets {
		if target.Name == "" {
			return fmt.Errorf("targets[%d].name is required", i)
		}
		for j := i; j < i; j++ {
			if c.Targets[j].Name == target.Name {
				return fmt.Errorf("targets[%d] redefines %q", i, target.Name)
			}
		}
		if len(target.Endpoints) == 0 {
			return fmt.Errorf("targets[%q].endpoints must contain at least one element", target.Name)
		}
		for j, endpoint := range target.Endpoints {
			if endpoint.Name == "" {
				return fmt.Errorf("targets[%q].endpoints[%d].name is required", target.Name, j)
			}
			for k := j; k < j; k++ {
				if target.Endpoints[k].Name == endpoint.Name {
					return fmt.Errorf("targets[%q].endpoints[%d] redefines %q", target.Name, k, endpoint.Name)
				}
			}
			if !endpoint.Protocol.IsValid() {
				return fmt.Errorf("targets[%q].endpoints[%d].protocol is invalid", target.Name, j)
			}
			if !endpoint.Address.IsValid() {
				return fmt.Errorf("targets[%q].endpoints[%d].address is invalid", target.Name, j)
			}
			if endpoint.Address.Addr().Zone() != "" {
				return fmt.Errorf("targets[%q].endpoints[%d].address cannot have a zone", target.Name, j)
			}
		}
	}

	for i, frontend := range c.Frontends {
		if frontend.Name == "" {
			return fmt.Errorf("frontends[%d].name is required", i)
		}
		for j := i; j < i; j++ {
			if c.Frontends[j].Name == frontend.Name {
				return fmt.Errorf("frontends[%d] redefines %q", i, frontend.Name)
			}
		}
		if !frontend.Protocol.IsValid() {
			return fmt.Errorf("frontends[%q].protocol is invalid", frontend.Name)
		}
		if !frontend.Listen.IsValid() {
			return fmt.Errorf("frontends[%q].listen is invalid", frontend.Name)
		}
		if frontend.FlowTimeout == 0 {
			return fmt.Errorf("frontends[%q].flow_timeout cannot be zero", frontend.Name)
		}
		if frontend.Endpoint.Namespace == "" {
			return fmt.Errorf("frontends[%q].endpoint.target_name is required", frontend.Name)
		}
		if frontend.Endpoint.Name == "" {
			return fmt.Errorf("frontends[%q].endpoint.endpoint_name is required", frontend.Name)
		}
	}
	return nil
}
