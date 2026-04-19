package method

// PluginInitRequest is sent by the host to initialize a plugin process.
type PluginInitRequest struct {
	Options map[string]any `json:"options,omitempty"`
}

// PluginInitResponse reports the kinds implemented by the plugin.
type PluginInitResponse struct {
	FrontendKinds []string `json:"frontend_kinds,omitempty"`
	TargetKinds   []string `json:"target_kinds,omitempty"`
}
