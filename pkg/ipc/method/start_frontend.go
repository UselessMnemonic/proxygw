package method

// StartFrontendRequest asks the plugin to start listening for a frontend.
type StartFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

func (StartFrontendRequest) Method() uint16 {
	return MethodStartFrontendRequest
}

type StartFrontendResponse struct{}

func (StartFrontendResponse) Method() uint16 {
	return MethodStartFrontendResponse
}
