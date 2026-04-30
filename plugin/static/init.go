package static

import (
	"github.com/UselessMnemonic/proxygw/pkg/engine"
	"github.com/UselessMnemonic/proxygw/plugin"
	"github.com/UselessMnemonic/proxygw/plugin/static/frontends"
	"github.com/UselessMnemonic/proxygw/plugin/static/targets"
)

func init() {
	ok := plugin.Register("static", plugin.Handler{
		OnLoad: func(_ map[string]any, _ *engine.Engine, namespace *plugin.Namespace) error {
			namespace.Frontends["always"] = frontends.NewAlwaysHandler
			namespace.Frontends["http"] = frontends.NewHTTPHandler
			namespace.Targets["cmd"] = targets.NewCmdHandler
			namespace.Targets["none"] = targets.NewNoneHandler
			return nil
		},
		OnUnload: func() error {
			return nil
		},
	})
	if !ok {
		panic("failed to register static")
	}
}
