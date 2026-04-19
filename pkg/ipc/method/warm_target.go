package method

// WarmTargetRequest asks the plugin to warm/activate a target.
type WarmTargetRequest struct {
	TargetID string `json:"target_id"`
}

type WarmTargetResponse struct{}
