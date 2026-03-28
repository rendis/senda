package e2e

import "testing"

func TestUseExternalStackEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "disabled without flag",
			env: map[string]string{
				"SENDA_BASE_URL":     "http://localhost:8090",
				"MAILPIT_URL":        "http://localhost:9025",
				"SENDA_DATABASE_URL": "postgres://localhost/senda",
			},
			want: false,
		},
		{
			name: "disabled when required url missing",
			env: map[string]string{
				"SENDA_E2E_EXTERNAL_STACK": "1",
				"SENDA_BASE_URL":           "http://localhost:8090",
				"SENDA_DATABASE_URL":       "postgres://localhost/senda",
			},
			want: false,
		},
		{
			name: "enabled with explicit true values and full wiring",
			env: map[string]string{
				"SENDA_E2E_EXTERNAL_STACK": "true",
				"SENDA_BASE_URL":           "http://localhost:8090",
				"MAILPIT_URL":              "http://localhost:9025",
				"SENDA_DATABASE_URL":       "postgres://localhost/senda",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := useExternalStackEnv(func(key string) string {
				return tc.env[key]
			})
			if got != tc.want {
				t.Fatalf("useExternalStackEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTruthyEnv(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"1", "true", "TRUE", " yes ", "On"} {
		if !truthyEnv(value) {
			t.Fatalf("truthyEnv(%q) = false, want true", value)
		}
	}

	for _, value := range []string{"", "0", "false", "off", "nah"} {
		if truthyEnv(value) {
			t.Fatalf("truthyEnv(%q) = true, want false", value)
		}
	}
}
