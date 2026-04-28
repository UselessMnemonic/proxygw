package static

import (
	"proxygw/pkg/engine"
	"proxygw/plugin"
	"proxygw/plugin/static/frontends"
	"proxygw/plugin/static/targets"
)

func init() {
	err := plugin.Register("static", plugin.Handler{
		OnLoad: func(_ map[string]any, _ *engine.Engine, namespace *plugin.Namespace) error {
			namespace.Frontends["eager"] = frontends.NewEagerHandler
			namespace.Frontends["http"] = frontends.NewHTTPHandler
			namespace.Targets["cmd"] = targets.NewCmdHandler
			namespace.Targets["none"] = targets.NewNoneHandler
			return nil
		},
		OnUnload: func() error {
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
}
