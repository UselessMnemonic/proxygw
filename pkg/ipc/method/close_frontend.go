package method

// CloseFrontendRequest asks the plugin to destroy a frontend driver instance.
type CloseFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

func (CloseFrontendRequest) Method() uint16 {
	return MethodCloseFrontendRequest
}

type CloseFrontendResponse struct{}

func (CloseFrontendResponse) Method() uint16 {
	return MethodCloseFrontendResponse
}
