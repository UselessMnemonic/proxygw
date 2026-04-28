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
)

const (
	MethodFrontendShouldWarmNotification uint16 = iota + 32768
)

// Method is implemented by every typed IPC payload.
type Method interface {
	Method() uint16
}

// IsResponse reports whether method is a response method ID.
func IsResponse(method uint16) bool {
	if method >= 32768 {
		return false
	}
	return method%2 == 1
}
