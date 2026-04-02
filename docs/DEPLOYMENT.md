# Senda Deployment Guide

## Production Build

```bash
docker build -f docker/Dockerfile -t senda:latest .
```

The Dockerfile uses a multi-stage build:

1. **Builder stage:** `golang:1.25-alpine` -- compiles the Go binary with `CGO_ENABLED=0` and strips debug symbols for a minimal, statically-linked output.
2. **Runtime stage:** `alpine:3.21` -- copies only the compiled binary and migration files into a minimal Alpine image.

The resulting image is small, contains no build tools, and runs as a non-root user.

## Required Environment Variables

These must be set for the application to start:

| Variable                   | Description                                   | Example                                                     |
| -------------------------- | --------------------------------------------- | ----------------------------------------------------------- |
| `SENDA_DATABASE_URL`       | PostgreSQL connection string                  | `postgres://user:pass@host:5432/senda?sslmode=require`      |
| `SENDA_OIDC_DISCOVERY_URL` | OIDC discovery endpoint                       | `https://auth.example.com/.well-known/openid-configuration` |
| `SENDA_OIDC_CLIENT_ID`     | OIDC client ID                                | `senda-web`                                                 |
| `SENDA_OIDC_CLIENT_SECRET` | OIDC client secret                            | _(secret)_                                                  |
| `SENDA_MASTER_KEY`         | AES-256-GCM encryption master key (32+ chars) | _(generate with `openssl rand -base64 32`)_                 |

The master key is used to encrypt sensitive data at rest (adapter credentials, API key hashes). Generate it once and store it securely. Rotating this key requires re-encrypting all sealed values.

## Optional Environment Variables

| Variable                  | Default      | Description                                      |
| ------------------------- | ------------ | ------------------------------------------------ |
| `SENDA_HOST`              | `0.0.0.0`    | Bind address                                     |
| `SENDA_PORT`              | `8080`       | HTTP port                                        |
| `SENDA_LOG_LEVEL`         | `info`       | Log level (`debug`, `info`, `warn`, `error`)     |
| `SENDA_LOG_FORMAT`        | `json`       | Log format (`json`, `text`)                      |
| `SENDA_SMTP_HOST`         | --           | SMTP server hostname (if using the SMTP adapter) |
| `SENDA_SMTP_PORT`         | `1025`       | SMTP server port                                 |
| `SENDA_TRACKING_BASE_URL` | --           | Public base URL for email tracking. Enables open-tracking pixels, SES ConfigSet/SNS auto-provisioning, and SNS webhook URL. Unset = tracking disabled, auto-provisioning returns 501 |
| `SENDA_MIGRATIONS_PATH`   | `migrations` | Path to SQL migration files inside the container |
| `SENDA_SNS_SKIP_SIGNATURE_VERIFICATION` | `false` | Skip SNS signature verification (test-only; do not enable in production) |

### AWS IAM Permissions for SES Adapters

**Sending (required)**:
- `ses:SendEmail`, `ses:SendRawEmail` -- Send emails
- `ses:ListEmailIdentities` -- Sync verified sender identities
- `ses:GetAccount` -- Check sandbox/production status

**Event Tracking Provisioning (required if `SENDA_TRACKING_BASE_URL` set)**:
- `ses:CreateConfigurationSet` -- Create tracking ConfigSet
- `ses:CreateConfigurationSetEventDestination` -- Link ConfigSet to SNS
- `ses:ListConfigurationSets` -- List existing ConfigSets
- `sns:CreateTopic` -- Create SNS Topic
- `sns:Subscribe` -- Subscribe webhook
- `sns:GetSubscriptionAttributes` -- Verify subscription confirmed
- `sns:ListTopics` -- List existing Topics

**Cleanup on Adapter Deletion (recommended)**:
- `ses:DeleteConfigurationSet` -- Remove ConfigSet on adapter delete
- `ses:DeleteConfigurationSetEventDestination` -- Remove EventDest on adapter delete
- `sns:Unsubscribe` -- Cancel subscription on adapter delete
- `sns:DeleteTopic` -- Remove Topic on adapter delete

Note: Without cleanup permissions, adapter deletion works but AWS resources remain orphaned.

## Database Setup

- **PostgreSQL 16+** is required.
- The **pg_cron** extension must be available (used for scheduled maintenance jobs such as partition creation and stale data cleanup).
- Migrations run automatically on application start by default (`migrate_on_start: true` in the config).
- To run migrations manually instead:
  ```bash
  migrate -database "$DATABASE_URL" -path /migrations up
  ```
- The database user needs `CREATE` and `SUPERUSER` (or at minimum `CREATE EXTENSION`) privileges for the initial setup to install pg_cron. After setup, these elevated privileges can be revoked.

### Recommended PostgreSQL Settings

For production workloads, consider tuning:

- `shared_buffers`: 25% of available RAM
- `work_mem`: 64MB+
- `maintenance_work_mem`: 256MB+
- `max_connections`: 100+ (Senda uses a connection pool via pgx)
- Enable `ssl` mode for all connections

## OIDC Provider

Senda works with any OpenID Connect compliant provider:

- **Keycloak** (used in development)
- **Auth0**
- **Okta**
- **Google Identity Platform**

### Required Configuration

- **Scopes:** `openid`, `profile`, `email`
- **Redirect URI:** `https://your-frontend-domain/api/auth/callback/oidc`
- **Grant type:** Authorization Code with PKCE

Ensure the OIDC provider is configured to include the user's email in the ID token claims. Senda uses the email claim for user identity resolution.

## Email Providers

Email providers are configured per adapter through the management UI, not via environment variables. Senda supports:

### Amazon SES

Configure the adapter with your AWS region and credentials. The sending identity (domain or email address) must be verified in SES before sending. For sandbox accounts, recipient addresses must also be verified. For non-production harnesses you can also provide an adapter-level `endpoint_url` to target a compatible SES endpoint such as the aws-sim/MiniStack test harness; leave it unset in production.

### Gmail

Configure the adapter with an OAuth2 client ID, client secret, and refresh token. The Gmail API has daily sending limits; check your Google Workspace tier for quota details.

### SMTP

Configure the adapter with the SMTP relay host and port. This is the most flexible option and works with any SMTP-compatible service (SendGrid, Postmark, Mailgun, self-hosted).

### Provider Selection

Each adapter is assigned to a template type. When sending an email, Senda resolves the adapter through the 3-level hierarchy (Global, Tenant, Workspace) and uses the configured provider for that template type.

## Health Checks

The application exposes three health endpoints:

```
GET /health     -> { "status": "ok" }           # Basic liveness check
GET /healthz    -> { "status": "ok" }           # Readiness check (verifies DB connectivity)
GET /metrics    -> Prometheus text format        # Metrics endpoint for scraping
```

- Use `/health` for load balancer liveness probes (fast, no dependencies).
- Use `/healthz` for readiness probes (confirms the application can serve requests).
- Use `/metrics` for Prometheus scraping (request counts, latencies, queue depth, etc.).

## Reverse Proxy

Senda should run behind a reverse proxy that handles TLS termination. Example with Caddy:

```
senda.example.com {
    reverse_proxy localhost:8080
}
```

Example with Nginx:

```nginx
server {
    listen 443 ssl;
    server_name senda.example.com;

    ssl_certificate     /etc/ssl/certs/senda.crt;
    ssl_certificate_key /etc/ssl/private/senda.key;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Docker Run Example

Minimal production deployment:

```bash
docker run -d \
  --name senda \
  --restart unless-stopped \
  -p 8080:8080 \
  -e SENDA_DATABASE_URL="postgres://senda:secretpass@db.example.com:5432/senda?sslmode=require" \
  -e SENDA_OIDC_DISCOVERY_URL="https://auth.example.com/realms/senda/.well-known/openid-configuration" \
  -e SENDA_OIDC_CLIENT_ID="senda-web" \
  -e SENDA_OIDC_CLIENT_SECRET="your-client-secret" \
  -e SENDA_MASTER_KEY="$(openssl rand -base64 32)" \
  -e SENDA_LOG_LEVEL="info" \
  senda:latest
```

## Docker Compose (Production)

For a self-contained deployment with PostgreSQL included:

```yaml
services:
  senda:
    image: senda:latest
    ports:
      - "8080:8080"
    environment:
      SENDA_DATABASE_URL: postgres://senda:secretpass@postgres:5432/senda?sslmode=disable
      SENDA_OIDC_DISCOVERY_URL: https://auth.example.com/.well-known/openid-configuration
      SENDA_OIDC_CLIENT_ID: senda-web
      SENDA_OIDC_CLIENT_SECRET: your-client-secret
      SENDA_MASTER_KEY: your-generated-master-key
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: senda
      POSTGRES_PASSWORD: secretpass
      POSTGRES_DB: senda
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U senda"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

volumes:
  pgdata:
```

## Security Considerations

- Always run behind TLS (terminate at the reverse proxy).
- Store `SENDA_MASTER_KEY` in a secrets manager (AWS Secrets Manager, Vault, etc.), not in plaintext config files.
- Use `sslmode=require` (or `verify-full`) for the database connection in production.
- Restrict the `/metrics` endpoint to internal networks only (not publicly accessible).
- Run the container as a non-root user (the Dockerfile already sets this up).
- Rotate OIDC client secrets according to your organization's policy.
