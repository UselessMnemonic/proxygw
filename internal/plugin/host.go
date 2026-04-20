package plugin

import (
	"context"
	"proxygw/pkg/config"
	"time"
)

const (
	pluginListenEnv    = "PLUGIN_LISTEN"
	defaultDialTimeout = 10 * time.Second
)

type Host struct {
}

func NewHost(ctx context.Context, manifest config.PluginManifest) (*Host, error) {

}
