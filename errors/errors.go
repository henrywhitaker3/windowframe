// Package errors
package errors

import (
	"errors"

	perrors "github.com/pkg/errors"
)

var (
	New    = errors.New
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
	Stack  = perrors.WithStack
	Errorf = perrors.Errorf
)
