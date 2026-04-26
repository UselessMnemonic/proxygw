package method

// WarmTargetRequest asks the plugin to warm/activate a target.
type WarmTargetRequest struct {
	Name string `json:"name"`
}

func (WarmTargetRequest) Method() uint16 {
	return MethodWarmTargetRequest
}

type WarmTargetResponse struct{}

func (WarmTargetResponse) Method() uint16 {
	return MethodWarmTargetResponse
}
