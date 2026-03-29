package main

import (
	"log/slog"
	"os"

	"github.com/senda-app/senda/sdk"
)

// @title           Senda API
// @version         1.0
// @description     Multi-tenant email orchestration API for Senda.
// @BasePath        /
// @securityDefinitions.apikey ManagementBearer
// @in              header
// @name            Authorization
// @description     OIDC bearer token sent as `Authorization: Bearer <jwt>`.
// @securityDefinitions.apikey WorkspaceAPIKeyBearer
// @in              header
// @name            Authorization
// @description     Workspace API key sent as `Authorization: Bearer senda_live_...`.
func main() {
	cfgPath := os.Getenv("SENDA_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	engine := sdk.NewWithConfig(cfgPath)

	if err := engine.Run(); err != nil {
		slog.Error("senda failed", "error", err)
		os.Exit(1)
	}
}
