package method

// StopFrontendRequest asks the plugin to stop listening for a frontend.
type StopFrontendRequest struct {
	Name string `json:"name"`
}

func (StopFrontendRequest) Method() uint16 {
	return MethodStopFrontendRequest
}

// StopFrontendResponse confirms the frontend stop request was accepted.
type StopFrontendResponse struct{}

func (StopFrontendResponse) Method() uint16 {
	return MethodStopFrontendResponse
}
