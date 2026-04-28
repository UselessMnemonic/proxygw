package method

// StartFrontendRequest asks the plugin to start listening for a frontend.
type StartFrontendRequest struct {
	Name string `json:"name"`
}

func (StartFrontendRequest) Method() uint16 {
	return MethodStartFrontendRequest
}

// StartFrontendResponse confirms the frontend start request was accepted.
type StartFrontendResponse struct{}

func (StartFrontendResponse) Method() uint16 {
	return MethodStartFrontendResponse
}
