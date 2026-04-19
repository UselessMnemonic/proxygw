package method

// StartFrontendRequest asks the plugin to start listening for a frontend.
type StartFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

type StartFrontendResponse struct{}
