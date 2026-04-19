package method

// RemoveTargetRequest asks the plugin to destroy a target driver instance.
type RemoveTargetRequest struct {
	TargetID string `json:"target_id"`
}

type RemoveTargetResponse struct{}
