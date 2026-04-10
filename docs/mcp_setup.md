
# Senda MCP Setup

Connect agents to Senda through the OpenAPI-backed `senda` MCP server.

## Prerequisite

```bash
go install github.com/rendis/mcp-openapi-proxy/cmd/mcp-openapi-proxy@latest
```

## Claude Code

`.mcp.json` in the repo root is the source of truth.

```bash
make dev
claude mcp list
```

## OpenAI Codex

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

## Authentication

### Management plane

Use OIDC login for `/api/v1/manage/...` and member-profile operations.

```bash
mcp-openapi-proxy login
mcp-openapi-proxy status
```

### Data plane

Export a raw workspace API key, for example:

```bash
export MCP_AUTH_WORKSPACEAPIKEYBEARER_TOKEN="senda_prod_..."
```

or

```bash
export MCP_AUTH_WORKSPACEAPIKEYBEARER_TOKEN="senda_test_..."
```

### External integration surface

When calling external integration endpoints, include:

```http
X-Senda-Environment: prod|test
```

and whatever token/headers the configured external auth method expects.

## Recommended workflow

1. discover endpoints with `senda_list_endpoints`
2. inspect one endpoint with `senda_describe_endpoint`
3. call it with `senda_call_endpoint`

Use the bundled skill `skills/senda/` for operational guidance and workflows.
