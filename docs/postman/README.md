
# Senda API - Postman Collection

Postman material for the Senda API.

## Files

| File | Description |
| --- | --- |
| `senda-api-v1.postman_collection.json` | Full API collection |
| `senda-local.postman_environment.json` | Local development environment |
| `senda-staging.postman_environment.json` | Staging placeholder environment |

## Authentication

### Management plane

Use OIDC bearer auth for `/api/v1/manage/...` and member-profile requests.

### Data plane

Use a raw workspace API key:

```http
Authorization: Bearer senda_prod_...
```

or

```http
Authorization: Bearer senda_test_...
```

The collection variable `api_key` should store the raw key returned when creating an API key.

### External integration surface

For external integration requests, include:

```http
X-Senda-Environment: prod|test
```

plus the profile-specific auth headers or token expected by the configured external auth method.

## Recommended order

1. onboarding status / setup
2. create tenant
3. create workspace
4. create adapter
5. create template type
6. create template and version
7. publish version
8. create API key
9. send email
10. inspect email history

For environment-specific operations, use the environment-scoped management routes.

## Notes

- `_system` is the tenant control workspace for sharing and defaults.
- Gmail sharing is adapter-level.
- SES sharing is email-identity-level.
- Test recipient policies and runtime reset are test-only behaviors.
- Exact version cloning uses `POST /templates/:template_id/versions/:version_id/clone`.
