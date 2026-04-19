package method

// DrainTargetRequest asks the plugin to drain/deactivate a target.
type DrainTargetRequest struct {
	TargetID string `json:"target_id"`
}

type DrainTargetResponse struct{}
