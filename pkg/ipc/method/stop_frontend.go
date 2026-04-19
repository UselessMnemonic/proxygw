package method

// StopFrontendRequest asks the plugin to stop listening for a frontend.
type StopFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

type StopFrontendResponse struct{}
