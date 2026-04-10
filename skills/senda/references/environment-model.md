
# Environment Model

Senda supports exactly two operational environments per logical workspace:

- `prod`
- `test`

## Core idea

A logical workspace is edited once for shared identity (`name`, `code`) but operates as two real environment-specific workspaces for runtime behavior.

Environment-specific state includes:

- templates / versions / locales
- adapters and identities
- injectors
- API keys
- metrics and business history
- workspace policies
- test-recipient safety settings

## Where environment comes from

### Management plane

Environment-scoped workspace operations use path routing:

`/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/...`

Shared workspace CRUD still exists without the environment segment:

`/api/v1/manage/tenants/:tenant_code/workspaces/...`

Use the shared routes for logical workspace identity. Use environment-scoped routes for environment runtime state.

### Data plane

Environment comes from the API key prefix:

- `senda_prod_...`
- `senda_test_...`

The send `ref` stays:

`tenant:workspace:templateType`

Do not add `prod` or `test` to `ref`.

### External integration surface

Environment comes from:

`X-Senda-Environment: prod|test`

Requests without that header are invalid.

## Test environment behavior

`test` is not cosmetic. It changes behavior.

### Test recipient policy

Workspace test env supports:

- `replace` (default)
- `append`

and a safe recipient list.

Template types in the test environment can override the workspace default recipient policy.

Rules:

- recipient policy is test-only
- prod must not expose these controls
- recipient addresses are validated and deduplicated

### Runtime reset

Only the test environment supports runtime reset:

`POST /api/v1/manage/environments/test/tenants/:tenant_code/workspaces/:workspace_code/runtime/reset`

It clears runtime/business history for that test workspace. It does not delete functional configuration such as templates, adapters, injectors, or API keys.

## Propagation into public contexts

Environment is propagated into public extension contexts.

### For send/init/injectors

Use:

- `injCtx.Environment()`

### For external integrations

Use:

- `req.Environment`

Avoid custom booleans like `is_test`; the enum is the source of truth.
