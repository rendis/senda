package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/rendis/senda/sdk"
)

func main() {
	cfgPath := os.Getenv("SENDA_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	engine := sdk.NewWithConfig(cfgPath)
	if os.Getenv("SENDA_E2E_ENABLE_CODE_INJECTORS") == "true" {
		slog.Info("registering e2e code injectors")
		engine.RegisterInjector(e2eStudentInjector{})
	} else {
		slog.Warn("e2e code injectors disabled")
	}

	if err := engine.Run(); err != nil {
		slog.Error("senda e2e failed", "error", err)
		os.Exit(1)
	}
}

type e2eStudentInjector struct{}

func (e2eStudentInjector) Code() string { return "student" }

func (e2eStudentInjector) Resolve() (sdk.ResolveFunc, []string) {
	return func(_ context.Context, _ *sdk.InjectorContext) (map[string]any, error) {
		return map[string]any{
			"name":   "Code Student",
			"age":    22,
			"locked": "CODE-SHOULD-NOT-WIN",
			"status": "code-status",
		}, nil
	}, nil
}

func (e2eStudentInjector) IsCritical() bool { return true }

func (e2eStudentInjector) Timeout() time.Duration { return 0 }
