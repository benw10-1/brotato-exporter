package exporterserverutil

// ResponseError
type ResponseError struct {
	err        error
	statusCode int
	message    string
}

// NewResponseError
func NewResponseError(err error, statusCode int, message string) *ResponseError {
	return &ResponseError{
		err:        err,
		statusCode: statusCode,
		message:    message,
	}
}

// Error
func (re *ResponseError) Error() string {
	if re.err == nil {
		return re.message
	}

	return re.err.Error()
}

// StatusCode
func (re *ResponseError) StatusCode() int {
	return re.statusCode
}

// Message
func (re *ResponseError) Message() string {
	return re.message
}
