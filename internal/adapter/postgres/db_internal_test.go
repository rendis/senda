package postgres

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rendis/senda/pkg/apperr"
)

func TestClassifyPgError_CheckViolationBecomesValidationError(t *testing.T) {
	err := classifyPgError(&pgconn.PgError{
		Code:           "23514",
		ConstraintName: "workspaces_code_format",
		Message:        "new row for relation \"workspaces\" violates check constraint \"workspaces_code_format\"",
	})
	if err == nil {
		t.Fatal("expected validation error for check violation")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", appErr.Code)
	}
}
