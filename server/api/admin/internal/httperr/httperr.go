// Package httperr defines the admin API's error model: domain errors that
// carry an HTTP status code, and the global handler that maps any error to
// the unified JSON error response.
package httperr

import (
	"context"
	"errors"
	"net/http"

	"github.com/hoywu/expo-ota/server/api/admin/internal/types"
	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/zeromicro/go-zero/core/logx"
)

// statusError is a domain error carrying the HTTP status code it should be
// rendered with. ToErrorResponse maps it to the unified error response.
type statusError struct {
	Code    int
	Message string
}

func (e *statusError) Error() string {
	return e.Message
}

// New creates an error that ToErrorResponse renders with the given HTTP
// status code and message.
func New(code int, message string) error {
	return &statusError{Code: code, Message: message}
}

// BadRequest wraps a request parse/validation error as a 400 error.
// Handlers use it on httpx.Parse failures so that ToErrorResponse does not
// treat them as internal errors.
func BadRequest(err error) error {
	return &statusError{Code: http.StatusBadRequest, Message: err.Error()}
}

// ToErrorResponse is the global error handler registered via
// httpx.SetErrorHandlerCtx. It maps domain errors to HTTP status codes and
// the unified JSON error body, and hides internal error details from clients.
func ToErrorResponse(ctx context.Context, err error) (int, any) {
	var statusErr *statusError
	if errors.As(err, &statusErr) {
		return statusErr.Code, &types.ErrorResp{
			Code:    http.StatusText(statusErr.Code),
			Message: statusErr.Message,
		}
	}

	if errors.Is(err, models.ErrNotFound) {
		return http.StatusNotFound, &types.ErrorResp{
			Code:    http.StatusText(http.StatusNotFound),
			Message: "resource not found",
		}
	}

	logx.WithContext(ctx).Errorf("internal server error: %v", err)

	return http.StatusInternalServerError, &types.ErrorResp{
		Code:    http.StatusText(http.StatusInternalServerError),
		Message: "internal server error",
	}
}
