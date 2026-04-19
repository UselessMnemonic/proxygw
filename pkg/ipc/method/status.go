package method

// StatusRequest asks the host runtime for status details.
type StatusRequest struct{}

func (StatusRequest) Method() uint16 {
	return MethodStatusRequest
}

// StatusResponse reports runtime plugin host state.
type StatusResponse struct {
	Plugins []PluginStatus `json:"plugins,omitempty"`
}

func (StatusResponse) Method() uint16 {
	return MethodStatusResponse
}

type PluginStatus struct {
	Name          string   `json:"name"`
	Executable    string   `json:"executable"`
	Connected     bool     `json:"connected"`
	FrontendKinds []string `json:"frontend_kinds,omitempty"`
	TargetKinds   []string `json:"target_kinds,omitempty"`
	LastError     string   `json:"last_error,omitempty"`
}
