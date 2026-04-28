package method

// NewTargetRequest asks the plugin to construct a target driver instance.
type NewTargetRequest struct {
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	Options map[string]any `json:"options,omitempty"`
}

func (NewTargetRequest) Method() uint16 {
	return MethodNewTargetRequest
}

// NewTargetResponse confirms the target driver was created.
type NewTargetResponse struct{}

func (NewTargetResponse) Method() uint16 {
	return MethodNewTargetResponse
}
