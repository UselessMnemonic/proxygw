package config

import (
	"net/netip"
	"strings"
	"testing"
)

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "missing plugins",
			mutate: func(c *Config) {
				c.Plugins = nil
			},
			wantErr: "plugins is required",
		},
		{
			name: "missing targets",
			mutate: func(c *Config) {
				c.Targets = nil
			},
			wantErr: "targets is required",
		},
		{
			name: "missing frontends",
			mutate: func(c *Config) {
				c.Frontends = nil
			},
			wantErr: "frontends is required",
		},
		{
			name: "duplicate target",
			mutate: func(c *Config) {
				c.Targets = append(c.Targets, c.Targets[0])
			},
			wantErr: `targets[1] redefines "backend"`,
		},
		{
			name: "missing target kind",
			mutate: func(c *Config) {
				c.Targets[0].Kind = NamespaceReference{}
			},
			wantErr: `targets["backend"].kind.namespace is required`,
		},
		{
			name: "missing target idle timeout",
			mutate: func(c *Config) {
				c.Targets[0].IdleTimeout = 0
			},
			wantErr: `targets["backend"].idle_timeout cannot be zero`,
		},
		{
			name: "duplicate endpoint",
			mutate: func(c *Config) {
				c.Targets[0].Endpoints = append(c.Targets[0].Endpoints, c.Targets[0].Endpoints[0])
			},
			wantErr: `targets["backend"].endpoints[1] redefines "http"`,
		},
		{
			name: "duplicate frontend",
			mutate: func(c *Config) {
				c.Frontends = append(c.Frontends, c.Frontends[0])
			},
			wantErr: `frontends[1] redefines "public-http"`,
		},
		{
			name: "missing frontend kind",
			mutate: func(c *Config) {
				c.Frontends[0].Kind = NamespaceReference{}
			},
			wantErr: `frontends["public-http"].kind.namespace is required`,
		},
		{
			name: "unknown frontend target",
			mutate: func(c *Config) {
				c.Frontends[0].Endpoint.Namespace = "missing"
			},
			wantErr: `frontends["public-http"].target "missing:http" does not reference an existing endpoint`,
		},
		{
			name: "unknown frontend endpoint",
			mutate: func(c *Config) {
				c.Frontends[0].Endpoint.Name = "missing"
			},
			wantErr: `frontends["public-http"].target "backend:missing" does not reference an existing endpoint`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig()
			tc.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func validConfig() *Config {
	return &Config{
		Version: ConfigVersionV1,
		Log: Log{
			Output: "stderr",
		},
		Plugins: map[string]map[string]any{
			"static": {},
		},
		Targets: []Target{
			{
				Name: "backend",
				Kind: NamespaceReference{
					Namespace: "static",
					Name:      "none",
				},
				IdleTimeout: TTL(300),
				Endpoints: []TargetEndpoint{
					{
						Name:     "http",
						Protocol: ProtocolTCP,
						Address:  netip.MustParseAddrPort("127.0.0.1:8080"),
					},
				},
			},
		},
		Frontends: []Frontend{
			{
				Name: "public-http",
				Kind: NamespaceReference{
					Namespace: "static",
					Name:      "always",
				},
				Protocol:    ProtocolTCP,
				Listen:      netip.MustParseAddrPort("0.0.0.0:80"),
				FlowTimeout: TTL(60),
				Endpoint: NamespaceReference{
					Namespace: "backend",
					Name:      "http",
				},
			},
		},
	}
}
