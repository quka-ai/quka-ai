package errors

import (
	"fmt"
	"net/http"
	"strings"
)

type CustomizedError struct {
	cause   error
	message string
	trace   []string
	wrap    error
	code    int
	data    map[string]interface{}
}

func (e *CustomizedError) WithData(data map[string]interface{}) *CustomizedError {
	e.data = data
	return e
}

func (e *CustomizedError) Code(c int) *CustomizedError {
	e.code = c
	return e
}

func (e *CustomizedError) GetCode() int {
	return e.code
}

func New(trace, message string, err error) *CustomizedError {
	code := http.StatusInternalServerError
	return &CustomizedError{
		cause:   err,
		message: message,
		trace:   []string{trace},
		code:    code,
	}
}

func (e *CustomizedError) Trace(trace string) *CustomizedError {
	e.trace = append(e.trace, trace)
	return e
}

func Wrap(err error, trace, message string) *CustomizedError {
	ce := &CustomizedError{
		cause:   err,
		message: message,
		trace:   []string{trace},
		wrap:    err,
	}
	if income, ok := err.(*CustomizedError); ok {
		ce.code = income.code
	}
	return ce
}

func Trace(trace string, err error) *CustomizedError {
	if ce, ok := err.(*CustomizedError); ok {
		ce.trace = append(ce.trace, trace)
		return ce
	}
	return Wrap(err, trace, err.Error())
}

func (e *CustomizedError) Message() string {
	if e.message == "" {
		return e.cause.Error()
	}
	return e.message
}

func (e *CustomizedError) Error() string {
	otherDetails := `""`
	if ce, ok := e.wrap.(*CustomizedError); ok {
		otherDetails = ce.Error()
	} else if e.wrap != nil {
		otherDetails = fmt.Sprint("\"", e.wrap.Error(), "\"")
	}
	return fmt.Sprintf(`{"trace":"%s","code":%d,"msg":"%s","error":"%v","wrapd":%s}`, strings.Join(e.trace, "->"), e.code, e.message, e.cause, otherDetails)
}
