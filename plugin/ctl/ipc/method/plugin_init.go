package method

// PluginInitRequest is sent by the host to initialize a plugin process.
type PluginInitRequest struct{}

func (PluginInitRequest) Method() uint16 {
	return MethodPluginInitRequest
}

// PluginInitResponse reports the kinds implemented by the plugin.
type PluginInitResponse struct {
	FrontendKinds []string `json:"frontend_kinds,omitempty"`
	TargetKinds   []string `json:"target_kinds,omitempty"`
}

func (PluginInitResponse) Method() uint16 {
	return MethodPluginInitResponse
}
