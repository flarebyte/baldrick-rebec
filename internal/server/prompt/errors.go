package prompt

import (
	"errors"

	toolingdao "github.com/flarebyte/baldrick-rebec/internal/dao/tooling"
)

// mapError maps internal errors to Connect error codes and user-friendly messages.
func mapError(err error) (code, message string) {
	if err == nil {
		return "ok", ""
	}
	switch {
	case errors.Is(err, toolingdao.ErrToolNotFound):
		return "not_found", "tool not found"
	case errors.Is(err, toolingdao.ErrSecretNotFound):
		return "not_found", "secret not found"
	}
	// Default
	return "internal", err.Error()
}
