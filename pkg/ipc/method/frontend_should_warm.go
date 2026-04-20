package method

// FrontendShouldWarmNotification tells the host that the frontend's target should be warmed.
type FrontendShouldWarmNotification struct {
	FrontendID string `json:"frontend_id"`
}

func (FrontendShouldWarmNotification) Method() uint16 {
	return MethodFrontendShouldWarmNotification
}
