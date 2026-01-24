package apperr

import (
	"errors"
)

type Error interface {
	Code() string
	Status() int
	Error() string
}

type err struct {
	status int
	code   string
}

func (e *err) Code() string  { return e.code }
func (e *err) Status() int   { return e.status }
func (e *err) Error() string { return e.code }

func New(status int, code string) Error {
	return &err{
		status: status,
		code:   code,
	}
}

func Is(err error, target Error) bool {
	var e Error
	return errors.As(err, &e) && e.Status() == target.Status() && e.Code() == target.Code()
}
