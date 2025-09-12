package common

import (
	"fmt"

	"github.com/pkg/errors"
)

var (
	ErrBadRequest    = errors.New("bad request")
	ErrValidation    = errors.New("validation error")
	ErrRequiredField = fmt.Errorf("%w: required field", ErrValidation)
	ErrNotEqual      = fmt.Errorf("%w: fileds must match", ErrValidation)

	ErrUnauth    = errors.New("unauthorised")
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")

	Stack = errors.WithStack
	Wrap  = errors.Wrap
)
