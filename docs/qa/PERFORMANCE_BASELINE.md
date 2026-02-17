# Performance Baseline — HT-37

**Date:** 2026-02-17
**Environment:** Local Docker (M-series Mac), single Senda instance, single PostgreSQL

## Response Time Baselines

### Management API (JWT-authenticated)

| Endpoint | Method | Typical Response | Notes |
|----------|--------|-----------------|-------|
| `/api/v1/onboarding/setup` | POST | <50ms | One-time setup |
| `/api/v1/manage/.../workspaces` | POST | <50ms | |
| `/api/v1/manage/.../adapters` | POST | <50ms | |
| `/api/v1/manage/.../domains` | POST | <50ms | + async DNS verification |
| `/api/v1/manage/.../template-types` | POST | <50ms | |
| `/api/v1/manage/.../templates` | POST | <50ms | |
| `/api/v1/manage/.../templates/:id/versions` | POST | <50ms | |
| `/api/v1/manage/.../versions/:id/publish` | POST | <50ms | 204 No Content |
| `/api/v1/manage/.../api-keys` | POST | <50ms | |
| `/api/v1/manage/.../injectors` | POST | <50ms | |
| `/api/v1/manage/.../template-types` | GET | <50ms | List |
| `/api/v1/manage/.../adapters` | GET | <50ms | List |
| `/api/v1/manage/.../emails` | GET | <50ms | Cursor-based pagination |

### Send API (API Key-authenticated)

| Endpoint | Method | Typical Response | Notes |
|----------|--------|-----------------|-------|
| `/api/v1/send` | POST | <50ms | Returns 202 (queued) |
| `/api/v1/send` (batch 5) | POST | <50ms | 5 recipients |
| `/api/v1/send` (batch 10,000) | POST | ~6s | C08 PayloadGigante |

### Async Processing

| Operation | Typical Duration | Notes |
|-----------|-----------------|-------|
| Single email delivery (queue → Mailpit) | 20-25s | First email after cold start; includes River polling interval |
| Subsequent email delivery | <1s | Worker already warm, no polling delay |
| Batch 5 emails (total) | <2s | After first email warm-up |

### Chaos Recovery

| Scenario | Recovery Time | Notes |
|----------|--------------|-------|
| C01 Provider restart | ~8s | SMTP adapter reconnects |
| C02 DB connection lost | ~2s | pgx reconnects |
| C03 Worker crash recovery | ~7s | River auto-retries failed jobs |
| C09 Cache invalidation race | ~3s | PG UNLOGGED table cache |

### Concurrency

| Test | Concurrent Requests | Result |
|------|-------------------|--------|
| C04 Concurrent send | 50 simultaneous | All 202 accepted |
| C05 Concurrent publish | 10 simultaneous | Race handled correctly |
| C06 Rate limiter under load | 150 burst | All accepted (within burst) |

## Notes

- First email after server start takes ~24s due to River worker polling interval (default 5s) + MJML compilation warmup.
- All management API endpoints respond in <50ms under no-load conditions.
- The 10,000-recipient test (C08) takes ~6s to accept and queue all jobs; actual delivery is async.
- These baselines are local Docker measurements, not production-representative. Production will have network latency, connection pooling, and different hardware characteristics.
