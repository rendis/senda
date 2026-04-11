
# MCP Workflows

This reference provides operational workflows for Senda MCP usage.

## Auth selection

### Management plane

Use OIDC bearer token for:

- `/api/v1/manage/...`
- `/api/v1/members/me`
- onboarding setup

### Data plane

Use raw API key for:

- `/api/v1/send`
- `/api/v1/send/batch`
- `/api/v1/emails...`

Prefixes:

- `senda_prod_...`
- `senda_test_...`

### External integration surface

Use the external profile routes and include:

- `X-Senda-Environment: prod|test`
- any required profile headers
- whatever token/header scheme the custom auth method expects

## Typical workflows

### 1) First-time setup

1. check onboarding status
2. run onboarding setup if needed
3. list tenants
4. create workspace(s)

### 2) Create a sending workspace

1. create logical workspace
2. list/get the environment-scoped workspace you want (`prod` or `test`)
3. create adapter
4. create template type
5. create template
6. create version
7. publish version
8. create API key

When step 6 involves content editing, preview/test-send, or a visual-builder draft, load:
- `references/template-builder-contract.md`

### 3) Send and inspect

1. use a raw API key bearer token
2. call `/api/v1/send` or `/api/v1/send/batch`
3. inspect `/api/v1/emails`
4. inspect `/api/v1/emails/:tracking_id/events`

### 4) Work with environment-scoped management state

Use routes like:

`/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/...`

Use these for:

- environment-specific workspace reads/updates
- runtime reset in `test`
- environment-specific resources such as templates/adapters/injectors/API keys

### 5) External builder/embed flow

1. bootstrap profile
2. bootstrap environment-specific embed path if needed
3. call `/session` on the scoped external route
4. use template/template-version/injector/policy endpoints according to granted capabilities

If the flow mutates template versions or locale content, load:
- `references/template-builder-contract.md`

## Operational reminders

- Use `senda_list_endpoints` when unsure which operation exists.
- Use `senda_describe_endpoint` before crafting a complex request body.
- Prefer management shared routes for logical workspace identity and environment-scoped routes for runtime state.
- External requests must include `X-Senda-Environment`.
- Do not put environment into `ref`.
- Preview requests use `{ "mjml": "..." }`; do not invent version-style payloads for preview. Load `references/template-builder-contract.md` when unsure.
