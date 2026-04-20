package method

const (
	MethodEmpty uint16 = iota
	MethodErrorResponse

	MethodStatusRequest
	MethodStatusResponse

	MethodPluginInitRequest
	MethodPluginInitResponse

	MethodNewTargetRequest
	MethodNewTargetResponse

	MethodWarmTargetRequest
	MethodWarmTargetResponse

	MethodDrainTargetRequest
	MethodDrainTargetResponse

	MethodCloseTargetRequest
	MethodCloseTargetResponse

	MethodNewFrontendRequest
	MethodNewFrontendResponse

	MethodStartFrontendRequest
	MethodStartFrontendResponse

	MethodStopFrontendRequest
	MethodStopFrontendResponse

	MethodCloseFrontendRequest
	MethodCloseFrontendResponse

	MethodFrontendShouldWarmNotification uint16 = 100
)

type Method interface {
	Method() uint16
}

func IsResponse(method uint16) bool {
	return method%2 == 0
}
