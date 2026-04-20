package method

// DrainTargetRequest asks the plugin to drain/deactivate a target.
type DrainTargetRequest struct {
	TargetID string `json:"target_id"`
}

func (DrainTargetRequest) Method() uint16 {
	return MethodDrainTargetRequest
}

type DrainTargetResponse struct{}

func (DrainTargetResponse) Method() uint16 {
	return MethodDrainTargetResponse
}
