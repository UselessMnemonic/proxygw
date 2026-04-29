package method

// CloseFrontendRequest asks the plugin to destroy a frontend handler instance.
type CloseFrontendRequest struct {
	Name string `json:"name"`
}

func (CloseFrontendRequest) Method() uint16 {
	return MethodCloseFrontendRequest
}

// CloseFrontendResponse confirms the frontend close request was accepted.
type CloseFrontendResponse struct{}

func (CloseFrontendResponse) Method() uint16 {
	return MethodCloseFrontendResponse
}
