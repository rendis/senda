package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	e := &AppError{Code: 404, Message: "tenant not found"}
	want := "tenant not found"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Error_WithWrapped(t *testing.T) {
	inner := fmt.Errorf("db error")
	e := &AppError{Code: 500, Message: "internal", Err: inner}
	want := "internal: db error"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("original")
	e := &AppError{Code: 500, Message: "wrapped", Err: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should allow errors.Is to match inner error")
	}
}

func TestNotFound(t *testing.T) {
	e := NotFound("tenant %q", "abc")
	if e.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusNotFound)
	}
	if e.Message != `tenant "abc"` {
		t.Errorf("Message = %q, want %q", e.Message, `tenant "abc"`)
	}
}

func TestConflict(t *testing.T) {
	e := Conflict("slug %q exists", "welcome")
	if e.Code != http.StatusConflict {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusConflict)
	}
}

func TestForbidden(t *testing.T) {
	e := Forbidden("no access")
	if e.Code != http.StatusForbidden {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusForbidden)
	}
}

func TestValidation(t *testing.T) {
	e := Validation("field required")
	if e.Code != http.StatusBadRequest {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusBadRequest)
	}
}

func TestBadRequest(t *testing.T) {
	e := BadRequest("invalid input")
	if e.Code != http.StatusBadRequest {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusBadRequest)
	}
}

func TestUnprocessableEntity(t *testing.T) {
	e := UnprocessableEntity("validation failed")
	if e.Code != http.StatusUnprocessableEntity {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusUnprocessableEntity)
	}
}

func TestInternal(t *testing.T) {
	e := Internal("server error")
	if e.Code != http.StatusInternalServerError {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusInternalServerError)
	}
}

func TestAppError_Wrap(t *testing.T) {
	inner := fmt.Errorf("domain error")
	e := NotFound("tenant %q not found", "abc").Wrap(inner)

	if e.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusNotFound)
	}
	if !errors.Is(e, inner) {
		t.Error("errors.Is should match the wrapped error after Wrap")
	}
	want := `tenant "abc" not found: domain error`
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Wrap_Chaining(t *testing.T) {
	inner := fmt.Errorf("original")
	e := Conflict("dup").Wrap(inner)

	// Wrap returns the same pointer for chaining.
	if e.Err != inner {
		t.Error("Wrap should set Err to the provided error")
	}
}

func TestWithErr(t *testing.T) {
	inner := fmt.Errorf("db timeout")
	e := WithErr(inner, http.StatusServiceUnavailable, "service %s unavailable", "db")

	if e.Code != http.StatusServiceUnavailable {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusServiceUnavailable)
	}
	if e.Message != "service db unavailable" {
		t.Errorf("Message = %q, want %q", e.Message, "service db unavailable")
	}
	if !errors.Is(e, inner) {
		t.Error("errors.Is should match the wrapped error from WithErr")
	}
}
