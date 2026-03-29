# Senda MCP Setup

Connect AI agents to the Senda email orchestration API using [mcp-openapi-proxy](https://github.com/rendis/mcp-openapi-proxy).

## Prerequisites

```bash
go install github.com/rendis/mcp-openapi-proxy/cmd/mcp-openapi-proxy@latest
```

## Claude Code

Auto-detected from `.mcp.json` in repo root. No manual setup needed.

```bash
make dev                    # start senda (port 8081)
claude mcp list             # verify "senda" connected
```

## OpenAI Codex

Edit `~/.codex/config.toml`:

```toml
[mcp_servers.senda]
command = "mcp-openapi-proxy"
args = []

[mcp_servers.senda.env]
MCP_SPEC = "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml"
MCP_BASE_URL = "http://localhost:8081"
MCP_TOOL_PREFIX = "senda"
```

## Gemini CLI

Edit `.gemini/settings.json` (project) or `~/.gemini/settings.json` (global):

```json
{
  "mcpServers": {
    "senda": {
      "command": "mcp-openapi-proxy",
      "args": [],
      "env": {
        "MCP_SPEC": "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml",
        "MCP_BASE_URL": "http://localhost:8081",
        "MCP_TOOL_PREFIX": "senda"
      }
    }
  }
}
```

## Production (OIDC)

Add OIDC config to env:

```
MCP_OIDC_ISSUER=https://auth.tether.education/auth/realms/tether-team
MCP_OIDC_CLIENT_ID=senda-web
MCP_AUTH_PROFILE=senda
```

Then authenticate:

```bash
mcp-openapi-proxy login     # browser-based OIDC PKCE
mcp-openapi-proxy status    # check token
mcp-openapi-proxy logout    # clear tokens
```

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `MCP_SPEC` | OpenAPI spec URL or file path | (required) |
| `MCP_BASE_URL` | API base URL | (required) |
| `MCP_TOOL_PREFIX` | Tool name prefix | `senda` |
| `MCP_AUTH_TOKEN` | Static bearer token (dev) | — |
| `MCP_OIDC_ISSUER` | OIDC provider URL | — |
| `MCP_OIDC_CLIENT_ID` | OIDC client ID | — |
| `MCP_AUTH_PROFILE` | Token storage namespace | `senda` |
