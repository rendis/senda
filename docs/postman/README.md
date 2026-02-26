# Senda API - Postman Collection

Comprehensive Postman collection for the Senda email orchestration platform API v1.

## Files

| File                                     | Description                                        |
| ---------------------------------------- | -------------------------------------------------- |
| `senda-api-v1.postman_collection.json`   | Full API collection (Postman v2.1 format)          |
| `senda-local.postman_environment.json`   | Environment for local development (localhost:8080) |
| `senda-staging.postman_environment.json` | Environment for staging (placeholder URL)          |

## Quick Start

1. **Import the collection** into Postman: File > Import > select `senda-api-v1.postman_collection.json`
2. **Import the environment**: File > Import > select `senda-local.postman_environment.json`
3. **Select the environment** from the environment dropdown (top-right)
4. **Set your OIDC token** in the environment variables (`oidc_token`)
5. **Start testing** with the Health folder to verify connectivity

## Authentication

### OIDC (Management API)

All management endpoints under `/api/v1/manage/` require an OIDC Bearer token. Set the `oidc_token` environment variable with a valid JWT.

The collection uses Bearer token auth by default. Individual requests inherit this.

### API Key (Data Plane)

The Send endpoint (`POST /api/v1/send`) uses API Key auth via the `X-API-Key` header.

To get an API key:

1. Create one via `POST /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/api-keys`
2. Save the `key` from the response (returned only once)
3. Set the `api_key` environment variable

### No Auth

- `GET /health`, `GET /healthz`, `GET /metrics` - public health checks
- `GET /api/v1/onboarding/status` - public onboarding status
- `POST /api/v1/webhooks/ses/inbound` - SNS signature verification (no app auth)

## Collection Structure

The collection is organized into folders matching the API structure:

| Folder             | Endpoints | Auth                     | Description                                |
| ------------------ | --------- | ------------------------ | ------------------------------------------ |
| **Health**         | 3         | None                     | Health checks and Prometheus metrics       |
| **Onboarding**     | 2         | Public / OIDC            | First-use setup                            |
| **Tenants**        | 5         | OIDC (superadmin)        | Tenant CRUD                                |
| **Workspaces**     | 5         | OIDC (tenant_admin+)     | Workspace CRUD                             |
| **Members**        | 4         | OIDC (superadmin)        | Member and role management                 |
| **Config**         | 2         | OIDC (superadmin)        | Global configuration                       |
| **Injectors**      | 4         | OIDC (workspace roles)   | Injector definitions and values            |
| **Adapters**       | 5         | OIDC (workspace roles)   | Email adapter CRUD                         |
| **Domains**        | 5         | OIDC (workspace roles)   | Domain registration and DKIM verification  |
| **Template Types** | 3         | OIDC (workspace roles)   | Template type CRUD                         |
| **Templates**      | 9         | OIDC (workspace roles)   | Templates, versions, locales, MJML preview |
| **Send**           | 1         | API Key / OIDC           | Send emails                                |
| **Emails**         | 3         | OIDC (workspace_viewer+) | Email query and event history              |
| **Suppression**    | 3         | OIDC (workspace roles)   | Suppression list management                |
| **Audit Log**      | 1         | OIDC (workspace_viewer+) | Workspace audit log                        |
| **Webhooks**       | 6         | OIDC (workspace roles)   | Webhook CRUD and test                      |
| **API Keys**       | 3         | OIDC (workspace_admin)   | API key management                         |
| **SES Webhooks**   | 1         | None (SNS sig)           | Provider event ingestion                   |
| **Global**         | 15        | OIDC (superadmin)        | Global-scoped resources                    |

## Variables

The collection uses these variables (auto-populated by test scripts):

| Variable           | Source                      | Description                   |
| ------------------ | --------------------------- | ----------------------------- |
| `base_url`         | Environment                 | Server base URL               |
| `oidc_token`       | Environment                 | OIDC Bearer token             |
| `api_key`          | Auto (Create API Key)       | API key for send endpoint     |
| `tenant_code`      | Auto (Create Tenant)        | Current tenant code           |
| `workspace_code`   | Auto (Create Workspace)     | Current workspace code        |
| `member_id`        | Auto (Create Member)        | Last created member ID        |
| `role_id`          | Auto (Add Role)             | Last created role ID          |
| `adapter_id`       | Auto (Create Adapter)       | Last created adapter ID       |
| `domain_id`        | Auto (Register Domain)      | Last created domain ID        |
| `template_type_id` | Auto (Create Template Type) | Last created template type ID |
| `template_id`      | Auto (Create Template)      | Last created template ID      |
| `version_id`       | Auto (Create Version)       | Last created version ID       |
| `tracking_id`      | Auto (Send Email)           | Last tracking ID from send    |
| `webhook_id`       | Auto (Create Webhook)       | Last created webhook ID       |
| `api_key_id`       | Auto (Create API Key)       | Last created API key ID       |

## Recommended Test Order

For a full end-to-end flow, run requests in this order:

1. Health > Simple Health
2. Onboarding > Get Onboarding Status
3. Onboarding > Run Onboarding Setup
4. Workspaces > Create Workspace
5. Adapters > Create Adapter
6. Domains > Register Domain
7. Template Types > Create Template Type
8. Templates > Create Template
9. Templates > Create Version
10. Templates > Publish Version
11. API Keys > Create API Key
12. Send > Send Email
13. Emails > List Emails
14. Emails > Get Email by Tracking ID

## Pagination

All list endpoints use cursor-based pagination:

- `limit` (query param): Number of items per page (default: 25, max: 100)
- `cursor` (query param): Cursor from previous response's `next_cursor`
- Response includes `next_cursor` and `has_more` fields

## Error Format

All errors follow the standard envelope:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": [
      {
        "field": "name",
        "message": "is required"
      }
    ],
    "request_id": "abc-123"
  }
}
```

Error codes: `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `VALIDATION_ERROR`, `RATE_LIMITED`, `INTERNAL_ERROR`
