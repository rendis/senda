package ses

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
)

func TestIsAccessDenied_ValidatorCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AccessDeniedException returns true",
			err: &smithy.GenericAPIError{
				Code:    "AccessDeniedException",
				Message: "not authorized",
				Fault:   smithy.FaultClient,
			},
			want: true,
		},
		{
			name: "AccessDenied returns true",
			err: &smithy.GenericAPIError{
				Code:    "AccessDenied",
				Message: "access denied",
				Fault:   smithy.FaultClient,
			},
			want: true,
		},
		{
			name: "UnauthorizedAccess returns true",
			err: &smithy.GenericAPIError{
				Code:    "UnauthorizedAccess",
				Message: "unauthorized",
				Fault:   smithy.FaultClient,
			},
			want: true,
		},
		{
			name: "NotFoundException returns false",
			err: &smithy.GenericAPIError{
				Code:    "NotFoundException",
				Message: "not found",
				Fault:   smithy.FaultClient,
			},
			want: false,
		},
		{
			name: "InvalidParameterException returns false",
			err: &smithy.GenericAPIError{
				Code:    "InvalidParameterException",
				Message: "invalid parameter",
				Fault:   smithy.FaultClient,
			},
			want: false,
		},
		{
			name: "plain error returns false",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "nil returns false",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAccessDenied(tt.err)
			if got != tt.want {
				t.Errorf("IsAccessDenied() = %v, want %v", got, tt.want)
			}
		})
	}
}
