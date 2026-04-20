package method

// StopFrontendRequest asks the plugin to stop listening for a frontend.
type StopFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

func (StopFrontendRequest) Method() uint16 {
	return MethodStopFrontendRequest
}

type StopFrontendResponse struct{}

func (StopFrontendResponse) Method() uint16 {
	return MethodStopFrontendResponse
}
