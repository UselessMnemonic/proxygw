package method

// WarmTargetRequest asks the plugin to warm/activate a target.
type WarmTargetRequest struct {
	Name string `json:"name"`
}

func (WarmTargetRequest) Method() uint16 {
	return MethodWarmTargetRequest
}

// WarmTargetResponse confirms the target warm request was accepted.
type WarmTargetResponse struct{}

func (WarmTargetResponse) Method() uint16 {
	return MethodWarmTargetResponse
}
