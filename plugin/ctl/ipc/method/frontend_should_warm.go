package method

// FrontendShouldWarmNotification tells the host that the frontend's target should be warmed.
type FrontendShouldWarmNotification struct {
	Name string `json:"name"`
}

func (FrontendShouldWarmNotification) Method() uint16 {
	return MethodFrontendShouldWarmNotification
}
