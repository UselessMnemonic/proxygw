package method

const (
	MethodEmpty uint16 = iota
	MethodErrorResponse

	MethodStatusRequest
	MethodStatusResponse
)

type Method interface {
	Method() uint16
}

func IsResponse[M Method](method M) bool {
	return method.Method()%2 == 0
}
