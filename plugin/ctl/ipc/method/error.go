package method

// ErrorResponse signals an error response.
type ErrorResponse struct {
	Message string `json:"msg"`
}

func (e ErrorResponse) Method() uint16 {
	return MethodErrorResponse
}

func (e ErrorResponse) Error() string {
	return e.Message
}
