package method

// RegisterTargetRequest creates a target driver instance in the plugin.
type RegisterTargetRequest struct {
	Kind     string            `json:"kind"`
	Options  map[string]any    `json:"options,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RegisterTargetResponse returns the opaque target handle.
type RegisterTargetResponse struct {
	TargetID string `json:"target_id"`
}
