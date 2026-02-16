package response

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ErrorResponse is the standard JSON error envelope for all API errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error code, message, and optional field-level details.
type ErrorDetail struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Details   []FieldError `json:"details,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// WriteError writes a standard error response with the given HTTP status code.
func WriteError(c *echo.Context, status int, code, message string, details ...FieldError) error {
	reqID, _ := c.Get("request_id").(string)
	return c.JSON(status, ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: reqID,
		},
	})
}

// HTTPErrorHandler is a custom Echo error handler that converts errors into
// the standard ErrorResponse format.
func HTTPErrorHandler(c *echo.Context, err error) {
	if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
		if resp.Committed {
			return
		}
	}

	code := http.StatusInternalServerError
	message := "internal server error"
	errCode := "INTERNAL_ERROR"

	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		if tmp := sc.StatusCode(); tmp != 0 {
			code = tmp
		}
	}

	var he *echo.HTTPError
	if errors.As(err, &he) {
		if he.Message != "" {
			message = he.Message
		}
		errCode = httpStatusToCode(code)
	}

	reqID, _ := c.Get("request_id").(string)
	_ = c.JSON(code, ErrorResponse{
		Error: ErrorDetail{
			Code:      errCode,
			Message:   message,
			RequestID: reqID,
		},
	})
}

func httpStatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	default:
		return "INTERNAL_ERROR"
	}
}
