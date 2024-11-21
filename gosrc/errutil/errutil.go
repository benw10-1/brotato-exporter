package errutil

import (
	"errors"
	"fmt"
	"runtime"
)

// StackError
type StackError struct {
	err   error
	stack []uintptr
}

// Unwrap
func (e *StackError) Unwrap() error {
	return e.err
}

// Error
func (e *StackError) Error() string {
	return fmt.Sprintf("%v\n%s", e.err, stacktrace(e.stack))
}

// stacktrace
func stacktrace(stack []uintptr) string {
	frames := runtime.CallersFrames(stack)
	var str string
	for {
		frame, more := frames.Next()
		str += fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}
	return str
}

// New makes a StackError from the given value. If that value is already an
// error then it will be used directly, if not, it will be passed to
// fmt.Errorf("%v"). The stacktrace will point to the line of code that
// called New.
func NewStackError(e interface{}) error {
	if e == nil {
		return nil
	}

	var err error

	stack := make([]uintptr, 1)
	length := runtime.Callers(2, stack[:])

	switch e := e.(type) {
	case string:
		err = errors.New(e)
	case *StackError:
		return &StackError{
			err:   e.err,
			stack: append(stack[:length], e.stack...),
		}
	case error:
		err = e
	default:
		err = fmt.Errorf("%v", e)
	}

	return &StackError{
		err:   err,
		stack: stack[:length],
	}
}

// NewStackErrorf
func NewStackErrorf(format string, args ...interface{}) error {
	return NewStackError(fmt.Sprintf(format, args...))
}
