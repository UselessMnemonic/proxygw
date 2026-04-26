package static

import (
	"proxygw/internal/engine"
	"proxygw/plugin"
	"proxygw/plugin/static/frontends"
	"proxygw/plugin/static/targets"
)

func init() {
	err := plugin.Register("static", plugin.Handler{
		OnLoad: func(_ map[string]any, _ *engine.Engine, namespace *plugin.Namespace) error {
			namespace.Frontends["drop"] = frontends.NewDropHandler
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
