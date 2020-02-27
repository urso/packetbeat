package v2

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urso/diag"
	"github.com/urso/sderr"
)

// LoaderError is returned by Loaders in case of failures.
type LoaderError struct {
	// Additional metadata for structure logging (if applicable)
	Diagnostics *diag.Context

	// Name of input/module that failed to load (if applicable)
	Name string

	// Reason why the loader failed. Can either be the cause reported by the
	// Plugin or some other indicator like ErrUnknown
	Reason error

	// (optional) Message to report in additon.
	Message string
}

// ErrUnknown indicates that the plugin type does not exist. Either
// because the 'type' setting name does not match the loaders expectations,
// or because the type is unknown.
var ErrUnknown = errors.New("unknown input type")

// IsUnknownInputError checks if an error value indicates an input load
// error because there is no existing plugin that can create the input.
func IsUnknownInputError(err error) bool { return sderr.Is(err, ErrUnknown) }

func failedInputName(err error) string {
	switch e := err.(type) {
	case *LoaderError:
		return e.Name
	default:
		return ""
	}
}

// Context returns the errors diagnostics if present
func (e *LoaderError) Context() *diag.Context { return e.Diagnostics }

// Unwrap returns the reason if present
func (e *LoaderError) Unwrap() error { return e.Reason }

// Error returns the errors string repesentation
func (e *LoaderError) Error() string {
	var buf strings.Builder

	if e.Message != "" {
		buf.WriteString(e.Message)
	} else if e.Name != "" {
		buf.WriteString("failed to load ")
		buf.WriteString(e.Name)
	}

	if e.Reason != nil {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		fmt.Fprintf(&buf, "%v", e.Reason)
	}

	if buf.Len() == 0 {
		return "<loader error>"
	}
	return buf.String()
}
