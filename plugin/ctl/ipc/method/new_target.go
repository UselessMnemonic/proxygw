package method

// NewTargetRequest asks the plugin to construct a target handler instance.
type NewTargetRequest struct {
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	Options map[string]any `json:"options,omitempty"`
}

func (NewTargetRequest) Method() uint16 {
	return MethodNewTargetRequest
}

// NewTargetResponse confirms the target handler was created.
type NewTargetResponse struct{}

func (NewTargetResponse) Method() uint16 {
	return MethodNewTargetResponse
}
