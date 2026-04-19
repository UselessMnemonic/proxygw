package method

// RemoveFrontendRequest asks the plugin to destroy a frontend driver instance.
type RemoveFrontendRequest struct {
	FrontendID string `json:"frontend_id"`
}

type RemoveFrontendResponse struct{}
