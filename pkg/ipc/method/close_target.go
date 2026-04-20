package method

// CloseTargetRequest asks the plugin to destroy a target driver instance.
type CloseTargetRequest struct {
	Name string `json:"name"`
}

func (CloseTargetRequest) Method() uint16 {
	return MethodCloseTargetRequest
}

type CloseTargetResponse struct{}

func (CloseTargetResponse) Method() uint16 {
	return MethodCloseTargetResponse
}
