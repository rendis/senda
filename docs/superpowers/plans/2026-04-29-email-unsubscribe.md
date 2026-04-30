# Email Unsubscribe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement RFC 8058 / Gmail-Yahoo-bulk-sender compliant email unsubscribe with three-tier suppression (global, workspace, per template_type), reversible preference center, and full E2E validation including UI interaction via chromedp on the live dev stack with Mailpit.

**Architecture:** HMAC-SHA256 token signed with a per-workspace key, embedded in `List-Unsubscribe` header (RFC 2369 + RFC 8058 one-click POST) and exposed as `{{ system.unsubscribe_url }}` / `{{ system.preferences_url }}` template variables. Send pipeline checks three suppression levels in cascade; first match blocks. Public unauthenticated backend endpoints under `/api/v1/u/...`. Public Next.js pages under `/u/[token]` outside the `(dashboard)` route group. Backend renders no HTML — frontend fetches state from backend and renders. E2E test orchestrates the full stack: docker mailpit captures the email, headers are validated against the live MIME, one-click POST is exercised, and chromedp drives the actual Next.js pages to verify UI rendering and click behaviour.

**Tech Stack:** Go 1.26 · Postgres 16 + pgcrypto · pgx v5 · Echo v5 · River · Next.js 16 · React 19 · Tailwind v4 · shadcn/ui · ky · next-intl · chromedp · Testcontainers · Mailpit.

---

## Out-of-scope for v1 (explicit)

To keep this plan focused and shippable, these are deliberately excluded:

- **`mailto:` leg** of `List-Unsubscribe` header. Requires inbound mail handling. Only the HTTPS leg is implemented (sufficient for Gmail/Yahoo bulk sender requirements).
- **Token rotation UI**. Workspace signing keys are generated once at workspace creation and rotation requires manual SQL.
- **Suppression of recipients across tenants**. The global suppression table already covers cross-workspace bounces/complaints; this plan does not change global behaviour.
- **Auto-injection of footer HTML into MJML**. The variables are exposed; template authors place them where they want. Header injection is automatic when `template_type.is_bulk = true`.

---

## File Structure

### New files (backend)

| Path | Responsibility |
|---|---|
| `migrations/000048_email_unsubscribe.up.sql` | Schema: enum value, workspace signing key, `is_bulk`, `template_type_subscription` table |
| `migrations/000048_email_unsubscribe.down.sql` | Rollback |
| `internal/unsubscribe/token.go` | HMAC-SHA256 token generation + verification |
| `internal/unsubscribe/token_test.go` | Token unit tests |
| `internal/domain/template_type_subscription.go` | Domain entity |
| `internal/service/unsubscribe.go` | `UnsubscribeService` orchestrating token verify + suppression writes |
| `internal/service/unsubscribe_test.go` | Service tests with manual mocks |
| `internal/adapter/postgres/template_type_subscription_store.go` | Postgres impl of `TemplateTypeSubscriptionStore` |
| `internal/adapter/postgres/template_type_subscription_store_test.go` | Integration test with TestContainer |
| `internal/http/handler/unsubscribe.go` | Echo handler |
| `internal/http/handler/unsubscribe_test.go` | Handler tests |
| `internal/http/request/unsubscribe.go` | Request DTOs |
| `internal/http/response/unsubscribe.go` | Response DTOs |
| `test/e2e/unsubscribe_e2e_test.go` | Backend E2E with Mailpit (build tag `e2e`) |
| `test/e2e/unsubscribe_ui_test.go` | Full-stack E2E with chromedp UI driving (build tag `e2e_local`) |

### Modified files (backend)

| Path | Change |
|---|---|
| `internal/domain/suppression.go` | Add `SuppressionUnsubscribe` constant |
| `internal/domain/template.go` | Add `IsBulk bool` to `TemplateType` |
| `internal/port/store.go` | Add `TemplateTypeSubscriptionStore` interface, `WorkspaceSigningKeyGetter` |
| `internal/service/variable_renderer.go` | Add `system` prefix support (new firma) |
| `internal/service/variable_renderer_test.go` | Cover new prefix |
| `internal/service/send_suppression.go` | Add per-template_type level to `SuppressionBatchEvaluator` |
| `internal/service/send_suppression_test.go` | Cover new level |
| `internal/service/send.go` | Pass `template_type_id` through to evaluator |
| `internal/service/template_type.go` | Expose `IsBulk` in Create/Update |
| `internal/adapter/river/send_worker.go` | Generate token, inject headers + system vars when bulk |
| `internal/adapter/postgres/workspace_store.go` | Generate signing key on Create; expose `GetSigningKey` |
| `internal/adapter/postgres/template_type_store.go` | Read/write `is_bulk` |
| `internal/http/server.go` | Wire `UnsubscribeHandler` |
| `internal/http/routes_public.go` | Register public unsubscribe routes |
| `internal/openapi/openapi.go` | Recognise unsubscribe request types |
| `internal/app/bootstrap.go` | Construct `UnsubscribeService`, pass key getter to send worker |

### New files (frontend)

| Path | Responsibility |
|---|---|
| `web/src/app/u/[token]/page.tsx` | Landing page: opt-out this event vs all |
| `web/src/app/u/[token]/preferences/page.tsx` | Preference center (per-type checkboxes) |
| `web/src/app/u/[token]/layout.tsx` | Minimal layout (no dashboard chrome) |
| `web/src/components/unsubscribe/unsubscribe-form.tsx` | Client component for landing radio + submit |
| `web/src/components/unsubscribe/preferences-form.tsx` | Client component for preference checkboxes |
| `web/src/lib/unsubscribe-api.ts` | Public fetch helpers (no auth) |
| `web/src/components/ui/checkbox.tsx` | Added via `shadcn add` |
| `web/src/components/ui/radio-group.tsx` | Added via `shadcn add` |
| `web/src/components/ui/alert.tsx` | Added via `shadcn add` |

### Modified files (frontend)

| Path | Change |
|---|---|
| `web/messages/en.json` | Add `unsubscribe.*` keys |
| `web/messages/es.json` | Add `unsubscribe.*` keys |

### Modified docs / skills

| Path | Change |
|---|---|
| `docs/EMAIL_FLOWS.md` | Add "Unsubscribe lifecycle" section |
| `docs/specs/SECURITY_CHECKLIST.md` | Add signing key + token verification entry |
| `skills/senda/references/sending-emails.md` | Document `is_bulk`, header injection, system vars |
| `skills/senda/references/building-a-template.md` | Document `{{ system.unsubscribe_url }}` and `{{ system.preferences_url }}` |
| `skills/senda/SKILL.md` | Update topology table if needed |

---

## Public URL contract (lock this in before coding)

- **Header `List-Unsubscribe` HTTPS leg**: `https://<base>/api/v1/u/{token}` — backend, idempotent POST, no body required, returns 200.
- **Header `List-Unsubscribe-Post`**: literal value `List-Unsubscribe=One-Click`.
- **Footer link "Unsubscribe"** (when used): `https://<base>/u/{token}` — Next.js page, browser flow, GETs context from `/api/v1/u/{token}`.
- **Footer link "Manage preferences"** (when used): `https://<base>/u/{token}/preferences` — Next.js page, GETs preferences from `/api/v1/u/{token}/preferences`.

`<base>` is the value of `cfg.Tracking.BaseURL` (existing config). Header value MUST start with `https://` per RFC.

### Token format

```
{base64url(payload_json)}.{base64url(hmac_sha256)}
```

Payload (compact JSON, sorted keys):

```json
{"e":"juan@ejemplo.com","eid":"01927e85-...","exp":1761408000,"iat":1729872000,"tt":"newsletter-mensual","ttn":"Newsletter mensual","v":1,"ws":"01927e80-..."}
```

- `v` schema version (currently 1)
- `ws` workspace UUID
- `tt` template_type slug (snapshot of slug at send time)
- `ttn` template_type display name (snapshot, used by frontend so renames don't break old links)
- `e` canonical recipient email (lowercased, no plus-tag, RFC 5321 form)
- `eid` source email UUID (for audit trail)
- `iat` issued-at unix seconds
- `exp` expiry unix seconds (12 months from issue)

Signature: `HMAC-SHA256(payload_bytes, workspace.unsubscribe_signing_key)`.

Verification: parse, check signature constant-time, check exp, fetch workspace, recompute HMAC with workspace's stored key.

### Public endpoints (backend, all under `/api/v1/u/`)

| Method | Path | Purpose | Auth | Body |
|---|---|---|---|---|
| `GET` | `/api/v1/u/:token` | Get context for landing page (workspace name, template_type display name, current state) | none | — |
| `POST` | `/api/v1/u/:token` | One-click opt-out from this template_type (RFC 8058) | none | empty or `{}` |
| `POST` | `/api/v1/u/:token/all` | Opt-out from ALL emails of this workspace | none | empty or `{}` |
| `GET` | `/api/v1/u/:token/preferences` | List template_types received in last 12 months + current state | none | — |
| `POST` | `/api/v1/u/:token/preferences` | Update subscription state for one or more types | none | `{"changes":[{"template_type_slug":"...", "subscribed":true}]}` |

Errors: `404` if token invalid/expired, `410` if signing key rotated (treat as expired), `500` for store errors.

---

## Task 1: Create branch and scaffold plan tracking

**Files:**
- (no files in this task)

- [ ] **Step 1: Create feature branch from main**

```bash
git checkout main
git pull --ff-only
git checkout -b feat/email-unsubscribe
```

- [ ] **Step 2: Verify clean working tree**

Run: `git status`
Expected: `nothing to commit, working tree clean`

- [ ] **Step 3: Verify Go and Postgres versions**

```bash
go version
docker compose -f docker/docker-compose.yml config | grep -i "image: postgres" | head -1
```

Expected: `go version go1.26.x` and Postgres 16 image.

- [ ] **Step 4: Confirm pgcrypto is available**

```bash
docker exec -i $(docker ps -qf "name=postgres") psql -U senda -d senda -c "SELECT extname FROM pg_extension WHERE extname='pgcrypto';" 2>/dev/null || echo "stack not running yet"
```

If stack not running yet, this check will run again in Task 2. The migration creates the extension if missing.

---

## Task 2: Database migration

**Files:**
- Create: `migrations/000048_email_unsubscribe.up.sql`
- Create: `migrations/000048_email_unsubscribe.down.sql`

- [ ] **Step 1: Write the up migration**

Path: `migrations/000048_email_unsubscribe.up.sql`

```sql
-- 1. Ensure pgcrypto for gen_random_bytes (idempotent).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 2. Add 'unsubscribe' value to suppression_reason enum.
ALTER TYPE suppression_reason ADD VALUE IF NOT EXISTS 'unsubscribe';

-- 3. Per-workspace HMAC signing key for unsubscribe tokens.
ALTER TABLE workspaces
    ADD COLUMN unsubscribe_signing_key BYTEA;

UPDATE workspaces
    SET unsubscribe_signing_key = gen_random_bytes(32)
    WHERE unsubscribe_signing_key IS NULL;

ALTER TABLE workspaces
    ALTER COLUMN unsubscribe_signing_key SET NOT NULL;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_unsubscribe_signing_key_len
        CHECK (length(unsubscribe_signing_key) = 32);

-- 4. Mark template_types as bulk (subject to unsubscribe headers + system vars).
ALTER TABLE template_types
    ADD COLUMN is_bulk BOOLEAN NOT NULL DEFAULT false;

-- 5. Per-(workspace, template_type, email) subscription state.
CREATE TABLE template_type_subscription (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id),
    template_type_id  UUID NOT NULL REFERENCES template_types(id),
    email             VARCHAR(255) NOT NULL,
    subscribed        BOOLEAN NOT NULL,
    source            TEXT NOT NULL CHECK (source IN ('recipient_optout','recipient_optin','admin')),
    source_email_id   UUID,
    actor_id          UUID,
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tts_unique UNIQUE (workspace_id, template_type_id, email)
);

-- Partial index for the hot path: send pipeline only cares about explicit opt-outs.
CREATE INDEX idx_tts_optout
    ON template_type_subscription (workspace_id, template_type_id, email)
    WHERE subscribed = false;

-- Index for preference center lookups (all rows for a recipient in a workspace).
CREATE INDEX idx_tts_recipient_lookup
    ON template_type_subscription (workspace_id, email, template_type_id);
```

- [ ] **Step 2: Write the down migration**

Path: `migrations/000048_email_unsubscribe.down.sql`

```sql
-- Drop subscription table and its indexes (cascade drops indexes).
DROP TABLE IF EXISTS template_type_subscription;

-- Drop is_bulk column.
ALTER TABLE template_types DROP COLUMN IF EXISTS is_bulk;

-- Drop signing key column and constraint.
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_unsubscribe_signing_key_len;
ALTER TABLE workspaces DROP COLUMN IF EXISTS unsubscribe_signing_key;

-- Note: enum values cannot be removed in Postgres without recreating the type.
-- The 'unsubscribe' value is left in place; it is harmless if unused.
```

- [ ] **Step 3: Apply migration**

```bash
make dev-down >/dev/null 2>&1 || true
make dev-stack &
DEV_PID=$!
# Wait for postgres health
until docker exec -i $(docker ps -qf "name=postgres" | head -1) pg_isready -U senda >/dev/null 2>&1; do sleep 1; done
make migrate-up
```

Expected: migration `000048` applied. The container will keep running for next tasks.

- [ ] **Step 4: Verify schema**

```bash
docker exec -i $(docker ps -qf "name=postgres" | head -1) psql -U senda -d senda -c "\d template_type_subscription"
docker exec -i $(docker ps -qf "name=postgres" | head -1) psql -U senda -d senda -c "\d workspaces" | grep unsubscribe
docker exec -i $(docker ps -qf "name=postgres" | head -1) psql -U senda -d senda -c "\d template_types" | grep is_bulk
docker exec -i $(docker ps -qf "name=postgres" | head -1) psql -U senda -d senda -c "SELECT enum_range(NULL::suppression_reason);"
```

Expected:
- `template_type_subscription` table exists with all columns.
- `workspaces` has `unsubscribe_signing_key bytea` NOT NULL.
- `template_types` has `is_bulk boolean` NOT NULL DEFAULT false.
- `suppression_reason` enum range includes `unsubscribe`.

- [ ] **Step 5: Verify down migration is reversible**

```bash
make migrate-down ARGS=1
docker exec -i $(docker ps -qf "name=postgres" | head -1) psql -U senda -d senda -c "\d template_type_subscription" 2>&1 | grep -i "did not find" && echo OK_DROPPED
make migrate-up
```

Expected: down drops the table; second `migrate-up` re-applies cleanly.

- [ ] **Step 6: Commit**

```bash
git add migrations/000048_email_unsubscribe.up.sql migrations/000048_email_unsubscribe.down.sql
git commit -m "feat(db): add unsubscribe schema (signing key, is_bulk, template_type_subscription)"
```

---

## Task 3: Domain types

**Files:**
- Modify: `internal/domain/suppression.go`
- Modify: `internal/domain/template.go`
- Create: `internal/domain/template_type_subscription.go`

- [ ] **Step 1: Write failing test for new SuppressionReason**

Path: `internal/domain/suppression_test.go` (create if not present)

```go
package domain

import "testing"

func TestSuppressionReason_Unsubscribe(t *testing.T) {
    if string(SuppressionUnsubscribe) != "unsubscribe" {
        t.Fatalf("SuppressionUnsubscribe constant must serialize as 'unsubscribe', got %q", SuppressionUnsubscribe)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestSuppressionReason_Unsubscribe -v`
Expected: FAIL with `undefined: SuppressionUnsubscribe`.

- [ ] **Step 3: Add the constant**

Edit `internal/domain/suppression.go`. Add to the `const ( ... SuppressionReason ... )` block:

```go
const (
    SuppressionHardBounce SuppressionReason = "hard_bounce"
    SuppressionComplaint  SuppressionReason = "complaint"
    SuppressionManual     SuppressionReason = "manual"
    SuppressionUnsubscribe SuppressionReason = "unsubscribe"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/ -run TestSuppressionReason_Unsubscribe -v`
Expected: PASS.

- [ ] **Step 5: Write failing test for IsBulk on TemplateType**

Append to `internal/domain/template_test.go` (or create the file if it does not exist):

```go
func TestTemplateType_IsBulk_DefaultsFalse(t *testing.T) {
    tt := TemplateType{Slug: "x"}
    if tt.IsBulk {
        t.Fatalf("zero-value TemplateType.IsBulk must be false")
    }
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestTemplateType_IsBulk_DefaultsFalse -v`
Expected: FAIL with `tt.IsBulk undefined`.

- [ ] **Step 7: Add IsBulk field to TemplateType struct**

Edit `internal/domain/template.go`. Insert before `CreatedAt`:

```go
type TemplateType struct {
    ID                  uuid.UUID
    WorkspaceID         *uuid.UUID
    Slug                string
    Name                string
    Description         *string
    AdapterID           *uuid.UUID
    SenderIdentityID    *uuid.UUID
    VariableSchema      map[string]any
    TestRecipientMode   *TestRecipientMode
    TestRecipientAddresses []string
    OwnerScope          string
    InheritedFromSystem bool
    IsBulk              bool
    CreatedAt           time.Time
    UpdatedAt           time.Time
    DeletedAt           *time.Time
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/domain/ -run TestTemplateType_IsBulk_DefaultsFalse -v`
Expected: PASS.

- [ ] **Step 9: Create TemplateTypeSubscription domain type**

Path: `internal/domain/template_type_subscription.go`

```go
package domain

import (
    "time"

    "github.com/google/uuid"
)

type SubscriptionSource string

const (
    SubscriptionSourceRecipientOptout SubscriptionSource = "recipient_optout"
    SubscriptionSourceRecipientOptin  SubscriptionSource = "recipient_optin"
    SubscriptionSourceAdmin           SubscriptionSource = "admin"
)

type TemplateTypeSubscription struct {
    ID              uuid.UUID
    WorkspaceID     uuid.UUID
    TemplateTypeID  uuid.UUID
    Email           string
    Subscribed      bool
    Source          SubscriptionSource
    SourceEmailID   *uuid.UUID
    ActorID         *uuid.UUID
    Notes           *string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

- [ ] **Step 10: Run domain package build**

Run: `go build ./internal/domain/...`
Expected: success.

- [ ] **Step 11: Commit**

```bash
git add internal/domain/suppression.go internal/domain/suppression_test.go \
        internal/domain/template.go internal/domain/template_test.go \
        internal/domain/template_type_subscription.go
git commit -m "feat(domain): add unsubscribe types (SuppressionUnsubscribe, IsBulk, TemplateTypeSubscription)"
```

---

## Task 4: Token signing & verification (`internal/unsubscribe`)

**Files:**
- Create: `internal/unsubscribe/token.go`
- Create: `internal/unsubscribe/token_test.go`

- [ ] **Step 1: Write failing tests covering generate + verify roundtrip and tamper detection**

Path: `internal/unsubscribe/token_test.go`

```go
package unsubscribe

import (
    "encoding/base64"
    "strings"
    "testing"
    "time"

    "github.com/google/uuid"
)

func testKey(t *testing.T) []byte {
    t.Helper()
    k := make([]byte, 32)
    for i := range k {
        k[i] = byte(i + 1)
    }
    return k
}

func TestGenerate_Verify_Roundtrip(t *testing.T) {
    key := testKey(t)
    now := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version:               1,
        WorkspaceID:           uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000001"),
        TemplateTypeSlug:      "newsletter-mensual",
        TemplateTypeName:      "Newsletter Mensual",
        Email:                 "juan@ejemplo.com",
        SourceEmailID:         uuid.MustParse("01927e85-aaaa-bbbb-cccc-000000000002"),
        IssuedAt:              now,
        ExpiresAt:             now.Add(365 * 24 * time.Hour),
    }
    tok, err := Generate(p, key)
    if err != nil {
        t.Fatalf("Generate: %v", err)
    }
    if !strings.Contains(tok, ".") {
        t.Fatalf("token must contain payload.signature separator, got %q", tok)
    }
    got, err := Verify(tok, key, now)
    if err != nil {
        t.Fatalf("Verify roundtrip: %v", err)
    }
    if got.Email != p.Email || got.TemplateTypeSlug != p.TemplateTypeSlug ||
        got.WorkspaceID != p.WorkspaceID || got.SourceEmailID != p.SourceEmailID {
        t.Fatalf("payload mismatch: got=%+v want=%+v", got, p)
    }
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
    key := testKey(t)
    now := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := Generate(p, key)
    parts := strings.Split(tok, ".")
    if len(parts) != 2 {
        t.Fatalf("expected 2 parts, got %d", len(parts))
    }
    bad := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte("badsig"))
    if _, err := Verify(bad, key, now); err == nil {
        t.Fatal("Verify must reject tampered signature")
    }
}

func TestVerify_RejectsTamperedPayload(t *testing.T) {
    key := testKey(t)
    now := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := Generate(p, key)
    parts := strings.Split(tok, ".")
    bad := base64.RawURLEncoding.EncodeToString([]byte(`{"e":"attacker@x.com","v":1}`)) + "." + parts[1]
    if _, err := Verify(bad, key, now); err == nil {
        t.Fatal("Verify must reject when payload is replaced but signature kept")
    }
}

func TestVerify_RejectsExpired(t *testing.T) {
    key := testKey(t)
    issued := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: issued, ExpiresAt: issued.Add(time.Hour),
    }
    tok, _ := Generate(p, key)
    later := issued.Add(2 * time.Hour)
    if _, err := Verify(tok, key, later); err == nil {
        t.Fatal("Verify must reject expired token")
    }
}

func TestVerify_RejectsWrongKey(t *testing.T) {
    key1 := testKey(t)
    key2 := make([]byte, 32)
    for i := range key2 {
        key2[i] = 0xAA
    }
    now := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := Generate(p, key1)
    if _, err := Verify(tok, key2, now); err == nil {
        t.Fatal("Verify must reject token signed with different key")
    }
}

func TestVerify_RejectsUnsupportedVersion(t *testing.T) {
    key := testKey(t)
    now := time.Unix(1729872000, 0).UTC()
    p := Payload{
        Version: 99, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := Generate(p, key)
    if _, err := Verify(tok, key, now); err == nil {
        t.Fatal("Verify must reject unsupported version")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/unsubscribe/... -v`
Expected: FAIL with `undefined: Payload`, `Generate`, `Verify`.

- [ ] **Step 3: Implement token.go**

Path: `internal/unsubscribe/token.go`

```go
package unsubscribe

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

const supportedVersion = 1

var (
    ErrMalformedToken      = errors.New("unsubscribe: malformed token")
    ErrInvalidSignature    = errors.New("unsubscribe: invalid signature")
    ErrExpired             = errors.New("unsubscribe: token expired")
    ErrUnsupportedVersion  = errors.New("unsubscribe: unsupported token version")
)

type Payload struct {
    Version          int       `json:"v"`
    WorkspaceID      uuid.UUID `json:"ws"`
    TemplateTypeSlug string    `json:"tt"`
    TemplateTypeName string    `json:"ttn"`
    Email            string    `json:"e"`
    SourceEmailID    uuid.UUID `json:"eid"`
    IssuedAt         time.Time `json:"-"`
    ExpiresAt        time.Time `json:"-"`
}

type wirePayload struct {
    Version          int       `json:"v"`
    WorkspaceID      uuid.UUID `json:"ws"`
    TemplateTypeSlug string    `json:"tt"`
    TemplateTypeName string    `json:"ttn"`
    Email            string    `json:"e"`
    SourceEmailID    uuid.UUID `json:"eid"`
    IssuedAt         int64     `json:"iat"`
    ExpiresAt        int64     `json:"exp"`
}

func Generate(p Payload, key []byte) (string, error) {
    if len(key) != 32 {
        return "", fmt.Errorf("unsubscribe: signing key must be 32 bytes, got %d", len(key))
    }
    if p.Version == 0 {
        p.Version = supportedVersion
    }
    w := wirePayload{
        Version:          p.Version,
        WorkspaceID:      p.WorkspaceID,
        TemplateTypeSlug: p.TemplateTypeSlug,
        TemplateTypeName: p.TemplateTypeName,
        Email:            p.Email,
        SourceEmailID:    p.SourceEmailID,
        IssuedAt:         p.IssuedAt.Unix(),
        ExpiresAt:        p.ExpiresAt.Unix(),
    }
    body, err := json.Marshal(&w)
    if err != nil {
        return "", fmt.Errorf("unsubscribe: marshal payload: %w", err)
    }
    mac := hmac.New(sha256.New, key)
    mac.Write(body)
    sig := mac.Sum(nil)
    return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func Verify(token string, key []byte, now time.Time) (Payload, error) {
    if len(key) != 32 {
        return Payload{}, fmt.Errorf("unsubscribe: signing key must be 32 bytes, got %d", len(key))
    }
    parts := strings.SplitN(token, ".", 2)
    if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
        return Payload{}, ErrMalformedToken
    }
    body, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil {
        return Payload{}, fmt.Errorf("%w: payload decode: %v", ErrMalformedToken, err)
    }
    sig, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return Payload{}, fmt.Errorf("%w: signature decode: %v", ErrMalformedToken, err)
    }
    mac := hmac.New(sha256.New, key)
    mac.Write(body)
    expected := mac.Sum(nil)
    if !hmac.Equal(sig, expected) {
        return Payload{}, ErrInvalidSignature
    }
    var w wirePayload
    if err := json.Unmarshal(body, &w); err != nil {
        return Payload{}, fmt.Errorf("%w: unmarshal: %v", ErrMalformedToken, err)
    }
    if w.Version != supportedVersion {
        return Payload{}, fmt.Errorf("%w: got v%d", ErrUnsupportedVersion, w.Version)
    }
    p := Payload{
        Version:          w.Version,
        WorkspaceID:      w.WorkspaceID,
        TemplateTypeSlug: w.TemplateTypeSlug,
        TemplateTypeName: w.TemplateTypeName,
        Email:            w.Email,
        SourceEmailID:    w.SourceEmailID,
        IssuedAt:         time.Unix(w.IssuedAt, 0).UTC(),
        ExpiresAt:        time.Unix(w.ExpiresAt, 0).UTC(),
    }
    if now.After(p.ExpiresAt) {
        return Payload{}, ErrExpired
    }
    return p, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/unsubscribe/... -v -race`
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/unsubscribe/
git commit -m "feat(unsubscribe): HMAC-SHA256 token generate/verify with replay and tamper protection"
```

---

## Task 5: Variable renderer — `system` prefix

**Files:**
- Modify: `internal/service/variable_renderer.go`
- Modify: `internal/service/variable_renderer_test.go`
- Modify: `internal/port/store.go` (or wherever `port.VariableRenderer` lives) — only if interface exists; otherwise skip

- [ ] **Step 1: Locate the renderer interface (if any)**

Run: `grep -rn "VariableRenderer\b" internal/port/ internal/service/ internal/adapter/ 2>/dev/null`

If `port.VariableRenderer` interface exists, capture its current signature. The signature change to add `systemVars map[string]string` must be propagated.

- [ ] **Step 2: Write failing test for system prefix**

Append to `internal/service/variable_renderer_test.go`:

```go
func TestRender_SystemPrefix(t *testing.T) {
    r := NewVariableRenderer()
    body := `Hi {{ event.name }}, manage at {{ system.preferences_url }} or unsubscribe at {{ system.unsubscribe_url }}.`
    injectors := map[string]map[string]any{}
    eventVars := map[string]any{"name": "Juan"}
    systemVars := map[string]string{
        "unsubscribe_url": "https://x.test/api/v1/u/abc",
        "preferences_url": "https://x.test/u/abc/preferences",
    }
    got, err := r.RenderWithSystem(body, injectors, eventVars, systemVars)
    if err != nil {
        t.Fatalf("RenderWithSystem: %v", err)
    }
    want := `Hi Juan, manage at https://x.test/u/abc/preferences or unsubscribe at https://x.test/api/v1/u/abc.`
    if got != want {
        t.Fatalf("\nwant: %s\ngot:  %s", want, got)
    }
}

func TestRender_SystemPrefix_MissingKeyResolvesEmpty(t *testing.T) {
    r := NewVariableRenderer()
    body := `[{{ system.unknown }}]`
    out, err := r.RenderWithSystem(body, nil, nil, nil)
    if err != nil {
        t.Fatalf("err: %v", err)
    }
    if out != `[]` {
        t.Fatalf("expected empty replacement, got %q", out)
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestRender_SystemPrefix -v`
Expected: FAIL — `RenderWithSystem` undefined.

- [ ] **Step 4: Implement the new method without breaking existing callers**

Edit `internal/service/variable_renderer.go`. Add a new method `RenderWithSystem` and have the existing `Render` delegate:

```go
func (r *VariableRenderer) Render(body string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
    return r.RenderWithSystem(body, injectors, eventVars, nil)
}

func (r *VariableRenderer) RenderWithSystem(
    body string,
    injectors map[string]map[string]any,
    eventVars map[string]any,
    systemVars map[string]string,
) (string, error) {
    return r.replace(body, injectors, eventVars, systemVars)
}
```

In the `replace` method's switch on `parts[0]`, add the `system` case (alongside the existing `event` and `injector` cases):

```go
case "system":
    if len(parts) != 2 || systemVars == nil {
        return ""
    }
    return systemVars[parts[1]]
```

The `parts` slice for `system.unsubscribe_url` is `["system", "unsubscribe_url"]` — length 2, no further nesting.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestRender_SystemPrefix -v -race`
Expected: PASS.

- [ ] **Step 6: Verify no existing renderer tests broke**

Run: `go test ./internal/service/ -run TestRender -v -race`
Expected: all renderer tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/variable_renderer.go internal/service/variable_renderer_test.go
git commit -m "feat(service): add {{ system.X }} prefix to variable renderer"
```

---

## Task 6: Workspace store — signing key

**Files:**
- Modify: `internal/adapter/postgres/workspace_store.go`
- Modify: `internal/port/store.go` (add interface method)
- Modify: `internal/adapter/postgres/workspace_store_test.go` (or its integration counterpart)

- [ ] **Step 1: Locate workspace store and existing Create signature**

Run: `grep -n "func.*Workspace.*Create\|func.*Create.*Workspace\|func .*WorkspaceStore.*Create" internal/adapter/postgres/workspace_store.go`

Capture the current Create method.

- [ ] **Step 2: Write failing integration test**

Append to `internal/adapter/postgres/workspace_store_test.go`:

```go
func TestWorkspaceStore_Create_PopulatesSigningKey(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test")
    }
    ctx, store := setupWorkspaceStoreTest(t) // existing helper

    tenantID := seedTenant(t, ctx) // existing helper
    ws := &domain.Workspace{
        ID:       uuid.Must(uuid.NewV7()),
        TenantID: tenantID,
        Code:     "test-key-ws",
        Name:     "Test Key WS",
    }
    if err := store.Create(ctx, ws); err != nil {
        t.Fatalf("Create: %v", err)
    }
    key, err := store.GetUnsubscribeSigningKey(ctx, ws.ID)
    if err != nil {
        t.Fatalf("GetUnsubscribeSigningKey: %v", err)
    }
    if len(key) != 32 {
        t.Fatalf("signing key must be 32 bytes, got %d", len(key))
    }
}
```

(If the test file uses different setup helpers, adapt: the goal is `Create` followed by reading the column.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestWorkspaceStore_Create_PopulatesSigningKey -v`
Expected: FAIL — `GetUnsubscribeSigningKey` undefined.

- [ ] **Step 4: Add interface method to port**

Edit `internal/port/store.go`. In the `WorkspaceStore` (or equivalent) interface, add:

```go
GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error)
```

- [ ] **Step 5: Implement in workspace_store.go**

In the Postgres `Create` method, change the INSERT to also generate the signing key. Before insertion, generate 32 random bytes (use `crypto/rand`) and bind into the SQL.

```go
import "crypto/rand"

func (s *WorkspaceStore) Create(ctx context.Context, w *domain.Workspace) error {
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return fmt.Errorf("workspace: generate signing key: %w", err)
    }
    _, err := s.db.Exec(ctx, `
        INSERT INTO workspaces (
            id, tenant_id, code, name, /* ... existing columns ... */, unsubscribe_signing_key
        ) VALUES ($1, $2, $3, $4, /* ... */, $N)
    `, w.ID, w.TenantID, w.Code, w.Name, /* ... */, key)
    return err
}

func (s *WorkspaceStore) GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error) {
    var key []byte
    err := s.db.QueryRow(ctx, `SELECT unsubscribe_signing_key FROM workspaces WHERE id = $1`, workspaceID).Scan(&key)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, domain.ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("workspace: load signing key: %w", err)
    }
    return key, nil
}
```

(Adapt to the actual `Create` SQL and column list. The key insight is: bind one extra column.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestWorkspaceStore_Create_PopulatesSigningKey -v`
Expected: PASS.

- [ ] **Step 7: Verify no existing workspace tests broke**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestWorkspaceStore -v`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/port/store.go internal/adapter/postgres/workspace_store.go internal/adapter/postgres/workspace_store_test.go
git commit -m "feat(workspace): generate per-workspace unsubscribe signing key on create"
```

---

## Task 7: TemplateTypeSubscription store

**Files:**
- Modify: `internal/port/store.go`
- Create: `internal/adapter/postgres/template_type_subscription_store.go`
- Create: `internal/adapter/postgres/template_type_subscription_store_test.go`

- [ ] **Step 1: Add interface to port**

Edit `internal/port/store.go`. Add:

```go
type TemplateTypeSubscriptionStore interface {
    Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error
    GetState(ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string) (*domain.TemplateTypeSubscription, error)
    ListOptOutsForRecipient(ctx context.Context, workspaceID uuid.UUID, email string) ([]*domain.TemplateTypeSubscription, error)
    BatchCheckOptOut(ctx context.Context, workspaceID uuid.UUID, templateTypeID uuid.UUID, emails []string) (map[string]struct{}, error)
}
```

- [ ] **Step 2: Write failing integration tests**

Path: `internal/adapter/postgres/template_type_subscription_store_test.go`

```go
//go:build integration

package postgres

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/rendis/senda/internal/domain"
)

func TestTTSStore_Upsert_InsertsThenUpdates(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := newPgxPool(t) // existing helper
    defer cleanup()

    wsID := seedWorkspace(t, ctx, pool)        // existing helper
    ttID := seedTemplateType(t, ctx, pool, wsID) // existing helper

    store := NewTemplateTypeSubscriptionStore(pool)

    sub := &domain.TemplateTypeSubscription{
        ID:             uuid.Must(uuid.NewV7()),
        WorkspaceID:    wsID,
        TemplateTypeID: ttID,
        Email:          "juan@ejemplo.com",
        Subscribed:     false,
        Source:         domain.SubscriptionSourceRecipientOptout,
    }
    if err := store.Upsert(ctx, sub); err != nil {
        t.Fatalf("Upsert insert: %v", err)
    }
    got, err := store.GetState(ctx, wsID, ttID, "juan@ejemplo.com")
    if err != nil {
        t.Fatalf("GetState: %v", err)
    }
    if got.Subscribed != false {
        t.Fatalf("expected Subscribed=false, got %v", got.Subscribed)
    }

    // Re-subscribe
    sub.Subscribed = true
    sub.Source = domain.SubscriptionSourceRecipientOptin
    if err := store.Upsert(ctx, sub); err != nil {
        t.Fatalf("Upsert update: %v", err)
    }
    got2, err := store.GetState(ctx, wsID, ttID, "juan@ejemplo.com")
    if err != nil {
        t.Fatalf("GetState 2: %v", err)
    }
    if got2.Subscribed != true {
        t.Fatalf("expected Subscribed=true after flip, got %v", got2.Subscribed)
    }
    if got2.ID != got.ID {
        t.Fatalf("Upsert must not create a new row; got two IDs %v != %v", got.ID, got2.ID)
    }
}

func TestTTSStore_BatchCheckOptOut(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := newPgxPool(t)
    defer cleanup()
    wsID := seedWorkspace(t, ctx, pool)
    ttID := seedTemplateType(t, ctx, pool, wsID)
    store := NewTemplateTypeSubscriptionStore(pool)

    optOut := &domain.TemplateTypeSubscription{
        ID:             uuid.Must(uuid.NewV7()),
        WorkspaceID:    wsID,
        TemplateTypeID: ttID,
        Email:          "out@x.com",
        Subscribed:     false,
        Source:         domain.SubscriptionSourceRecipientOptout,
    }
    optIn := &domain.TemplateTypeSubscription{
        ID:             uuid.Must(uuid.NewV7()),
        WorkspaceID:    wsID,
        TemplateTypeID: ttID,
        Email:          "in@x.com",
        Subscribed:     true,
        Source:         domain.SubscriptionSourceRecipientOptin,
    }
    _ = store.Upsert(ctx, optOut)
    _ = store.Upsert(ctx, optIn)

    res, err := store.BatchCheckOptOut(ctx, wsID, ttID, []string{"out@x.com", "in@x.com", "never@x.com"})
    if err != nil {
        t.Fatalf("BatchCheckOptOut: %v", err)
    }
    if _, ok := res["out@x.com"]; !ok {
        t.Fatal("out@x.com must be opted-out")
    }
    if _, ok := res["in@x.com"]; ok {
        t.Fatal("in@x.com is opted-in, must NOT appear")
    }
    if _, ok := res["never@x.com"]; ok {
        t.Fatal("never@x.com has no row, must NOT appear")
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestTTSStore -v`
Expected: FAIL — `NewTemplateTypeSubscriptionStore` undefined.

- [ ] **Step 4: Implement the store**

Path: `internal/adapter/postgres/template_type_subscription_store.go`

```go
package postgres

import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rendis/senda/internal/domain"
)

type TemplateTypeSubscriptionStore struct {
    db *pgxpool.Pool
}

func NewTemplateTypeSubscriptionStore(db *pgxpool.Pool) *TemplateTypeSubscriptionStore {
    return &TemplateTypeSubscriptionStore{db: db}
}

func (s *TemplateTypeSubscriptionStore) Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error {
    _, err := s.db.Exec(ctx, `
        INSERT INTO template_type_subscription
            (id, workspace_id, template_type_id, email, subscribed, source, source_email_id, actor_id, notes)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (workspace_id, template_type_id, email) DO UPDATE
            SET subscribed      = EXCLUDED.subscribed,
                source          = EXCLUDED.source,
                source_email_id = EXCLUDED.source_email_id,
                actor_id        = EXCLUDED.actor_id,
                notes           = EXCLUDED.notes,
                updated_at      = now()
    `,
        sub.ID, sub.WorkspaceID, sub.TemplateTypeID, sub.Email,
        sub.Subscribed, sub.Source, sub.SourceEmailID, sub.ActorID, sub.Notes,
    )
    if err != nil {
        return fmt.Errorf("tts: upsert: %w", err)
    }
    return nil
}

func (s *TemplateTypeSubscriptionStore) GetState(
    ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string,
) (*domain.TemplateTypeSubscription, error) {
    var sub domain.TemplateTypeSubscription
    err := s.db.QueryRow(ctx, `
        SELECT id, workspace_id, template_type_id, email, subscribed, source,
               source_email_id, actor_id, notes, created_at, updated_at
        FROM template_type_subscription
        WHERE workspace_id = $1 AND template_type_id = $2 AND email = $3
    `, workspaceID, templateTypeID, email).Scan(
        &sub.ID, &sub.WorkspaceID, &sub.TemplateTypeID, &sub.Email,
        &sub.Subscribed, &sub.Source,
        &sub.SourceEmailID, &sub.ActorID, &sub.Notes,
        &sub.CreatedAt, &sub.UpdatedAt,
    )
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, domain.ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("tts: get state: %w", err)
    }
    return &sub, nil
}

func (s *TemplateTypeSubscriptionStore) ListOptOutsForRecipient(
    ctx context.Context, workspaceID uuid.UUID, email string,
) ([]*domain.TemplateTypeSubscription, error) {
    rows, err := s.db.Query(ctx, `
        SELECT id, workspace_id, template_type_id, email, subscribed, source,
               source_email_id, actor_id, notes, created_at, updated_at
        FROM template_type_subscription
        WHERE workspace_id = $1 AND email = $2
    `, workspaceID, email)
    if err != nil {
        return nil, fmt.Errorf("tts: list: %w", err)
    }
    defer rows.Close()
    var out []*domain.TemplateTypeSubscription
    for rows.Next() {
        var sub domain.TemplateTypeSubscription
        if err := rows.Scan(
            &sub.ID, &sub.WorkspaceID, &sub.TemplateTypeID, &sub.Email,
            &sub.Subscribed, &sub.Source,
            &sub.SourceEmailID, &sub.ActorID, &sub.Notes,
            &sub.CreatedAt, &sub.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("tts: scan: %w", err)
        }
        out = append(out, &sub)
    }
    return out, rows.Err()
}

func (s *TemplateTypeSubscriptionStore) BatchCheckOptOut(
    ctx context.Context, workspaceID, templateTypeID uuid.UUID, emails []string,
) (map[string]struct{}, error) {
    if len(emails) == 0 {
        return map[string]struct{}{}, nil
    }
    rows, err := s.db.Query(ctx, `
        SELECT email
        FROM template_type_subscription
        WHERE workspace_id = $1
          AND template_type_id = $2
          AND email = ANY($3)
          AND subscribed = false
    `, workspaceID, templateTypeID, emails)
    if err != nil {
        return nil, fmt.Errorf("tts: batch check: %w", err)
    }
    defer rows.Close()
    out := make(map[string]struct{})
    for rows.Next() {
        var e string
        if err := rows.Scan(&e); err != nil {
            return nil, fmt.Errorf("tts: scan email: %w", err)
        }
        out[e] = struct{}{}
    }
    return out, rows.Err()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestTTSStore -v`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/port/store.go internal/adapter/postgres/template_type_subscription_store.go internal/adapter/postgres/template_type_subscription_store_test.go
git commit -m "feat(store): TemplateTypeSubscription Postgres store with upsert and batch opt-out check"
```

---

## Task 8: SuppressionBatchEvaluator — third level

**Files:**
- Modify: `internal/service/send_suppression.go`
- Modify: `internal/service/send_suppression_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/service/send_suppression_test.go`:

```go
type fakeTTSStore struct {
    optOuts map[string]struct{} // email set
}

func (f *fakeTTSStore) BatchCheckOptOut(ctx context.Context, _ uuid.UUID, _ uuid.UUID, emails []string) (map[string]struct{}, error) {
    out := make(map[string]struct{})
    for _, e := range emails {
        if _, ok := f.optOuts[e]; ok {
            out[e] = struct{}{}
        }
    }
    return out, nil
}

func TestEvaluator_SkipsRecipientsOptedOutOfTemplateType(t *testing.T) {
    ctx := context.Background()
    wsStore := &fakeWsSuppressionStore{}                                // existing fake, returns no suppressions
    ttsStore := &fakeTTSStore{optOuts: map[string]struct{}{"a@x.com": {}}}
    eval := NewSuppressionBatchEvaluator(wsStore).WithTemplateTypeStore(ttsStore)

    wsID := uuid.New()
    ttID := uuid.New()
    res, err := eval.EvaluateForType(ctx, wsID, ttID,
        []string{"a@x.com", "b@x.com"}, nil, nil)
    if err != nil {
        t.Fatalf("Evaluate: %v", err)
    }
    if len(res.To) != 2 {
        t.Fatalf("expected 2 To decisions, got %d", len(res.To))
    }
    var aDec, bDec SuppressionRecipientDecision
    for _, d := range res.To {
        if d.Address == "a@x.com" { aDec = d }
        if d.Address == "b@x.com" { bDec = d }
    }
    if !aDec.Suppressed || aDec.Reason != "type_optout" {
        t.Fatalf("a@x.com must be suppressed with reason=type_optout, got %+v", aDec)
    }
    if bDec.Suppressed {
        t.Fatalf("b@x.com must not be suppressed, got %+v", bDec)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestEvaluator_SkipsRecipientsOptedOut -v`
Expected: FAIL — `WithTemplateTypeStore`, `EvaluateForType` undefined.

- [ ] **Step 3: Extend the evaluator**

Edit `internal/service/send_suppression.go`:

```go
type templateTypeOptOutStore interface {
    BatchCheckOptOut(ctx context.Context, workspaceID, templateTypeID uuid.UUID, emails []string) (map[string]struct{}, error)
}

func (e *SuppressionBatchEvaluator) WithTemplateTypeStore(ts templateTypeOptOutStore) *SuppressionBatchEvaluator {
    e.tts = ts
    return e
}

// Add field to SuppressionBatchEvaluator struct:
//   tts templateTypeOptOutStore

// EvaluateForType performs the standard workspace-level evaluation, then layers
// per-(template_type, email) opt-outs on top. Recipients opted out of this
// type are marked Suppressed with Reason="type_optout" while still receiving
// any global/workspace suppression reason if present (workspace wins).
func (e *SuppressionBatchEvaluator) EvaluateForType(
    ctx context.Context,
    workspaceID, templateTypeID uuid.UUID,
    to, cc, bcc []string,
) (*SuppressionBatchEvaluation, error) {
    base, err := e.Evaluate(ctx, workspaceID, to, cc, bcc)
    if err != nil {
        return nil, err
    }
    if e.tts == nil {
        return base, nil
    }

    canonical := make([]string, 0, len(base.To))
    for _, d := range base.To {
        if !d.Suppressed {
            canonical = append(canonical, domain.CanonicalRecipientAddress(d.Address))
        }
    }
    optOuts, err := e.tts.BatchCheckOptOut(ctx, workspaceID, templateTypeID, canonical)
    if err != nil {
        return nil, fmt.Errorf("evaluate type opt-outs: %w", err)
    }

    for i, d := range base.To {
        if d.Suppressed {
            continue
        }
        if _, ok := optOuts[domain.CanonicalRecipientAddress(d.Address)]; ok {
            base.To[i].Suppressed = true
            base.To[i].Reason = "type_optout"
        }
    }
    // CC/BCC: filter out type-opt-outs.
    base.CC = filterCanonical(base.CC, optOuts)
    base.BCC = filterCanonical(base.BCC, optOuts)
    return base, nil
}

func filterCanonical(addrs []string, optOuts map[string]struct{}) []string {
    out := make([]string, 0, len(addrs))
    for _, a := range addrs {
        if _, ok := optOuts[domain.CanonicalRecipientAddress(a)]; ok {
            continue
        }
        out = append(out, a)
    }
    return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestEvaluator -v -race`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/send_suppression.go internal/service/send_suppression_test.go
git commit -m "feat(send): add per-template_type opt-out level to suppression evaluator"
```

---

## Task 9: Send pipeline integration

**Files:**
- Modify: `internal/adapter/river/send_worker.go`
- Modify: `internal/service/send.go` (and/or `send_batch.go` if needed)
- Modify: `internal/app/bootstrap.go`
- Modify: `internal/adapter/river/send_worker_test.go` (if exists; otherwise add)

This task wires the evaluator's new method into the send code path and injects the headers + system vars in the worker. Two integration points:

1. **`send.go` / `send_batch.go`** — must call `EvaluateForType(workspaceID, templateTypeID, ...)` instead of the current `Evaluate(workspaceID, ...)`. The `templateTypeID` comes from the resolved template.
2. **`send_worker.go`** — when `template_type.is_bulk == true`, generate a token and add headers + system vars before sending.

- [ ] **Step 1: Locate current Evaluate call site in send pipeline**

Run: `grep -rn "\.Evaluate(\|SuppressionBatchEvaluator" internal/service/ 2>/dev/null`

Document the current call site (file:line). Plan to swap it for `EvaluateForType` with the resolved `template_type_id`.

- [ ] **Step 2: Wire EvaluateForType in send.go**

Edit `internal/service/send.go`. Where `evaluator.Evaluate(ctx, wsID, to, cc, bcc)` is currently called, replace with:

```go
res, err := s.suppressionEvaluator.EvaluateForType(
    ctx, resolved.Workspace.ID, resolved.TemplateType.ID,
    to, cc, bcc,
)
```

If the evaluator is constructed without `WithTemplateTypeStore`, the new method falls back to the legacy behaviour — backward compatible.

- [ ] **Step 3: Wire bootstrap to attach the new store**

Edit `internal/app/bootstrap.go`. Where `SuppressionBatchEvaluator` is constructed, chain `.WithTemplateTypeStore(repos.templateTypeSubscriptionStore)` and ensure the store is constructed in the repo wiring block (`NewTemplateTypeSubscriptionStore(infra.pgxPool)`).

```go
// In repo wiring:
templateTypeSubscriptionStore: postgres.NewTemplateTypeSubscriptionStore(infra.pgxPool),

// In service construction:
suppressionEvaluator := service.NewSuppressionBatchEvaluator(repos.suppressionStore).
    WithTemplateTypeStore(repos.templateTypeSubscriptionStore)
```

- [ ] **Step 4: Add `is_bulk` and signing key to send-worker payload**

The worker must know two things to inject the header:
1. Whether the template is bulk (current `email` row needs the flag).
2. The workspace's signing key.

The simplest and cheapest approach is to pass the signing key getter and the template type store into `SendWorker` via constructor options, and have the worker resolve `is_bulk` via a quick lookup keyed by `(workspace_id, template_type_slug)`. This avoids denormalising onto the (partitioned) `emails` table.

Edit `internal/adapter/river/send_worker.go`. Add fields to the `SendWorker` struct:

```go
type SendWorker struct {
    // ... existing fields ...
    templateTypeStore   port.TemplateTypeStore
    workspaceKeyGetter  port.WorkspaceSigningKeyGetter
    publicBaseURL       string
}
```

Add `WithTemplateTypeStore`, `WithWorkspaceKeyGetter`, `WithPublicBaseURL` options. The `publicBaseURL` reuses `cfg.Tracking.BaseURL`.

In `bootstrap.go` (`internal/app/bootstrap.go:262-264` area), add the new options when constructing the worker:

```go
sendWorkerOpts = append(sendWorkerOpts,
    river.WithTemplateTypeStore(repos.templateTypeStore),
    river.WithWorkspaceKeyGetter(repos.workspaceStore),
    river.WithPublicBaseURL(cfg.Tracking.BaseURL),
)
```

- [ ] **Step 5: Generate token and inject headers in the worker**

Edit `internal/adapter/river/send_worker.go`. Between step 6 (compile MJML) and step 8 (build outgoing email), insert:

```go
// 6.5 — Resolve template_type to know if this is bulk and to snapshot its name.
var systemVars map[string]string
extraHeaders := map[string]string{}
if w.templateTypeStore != nil && w.workspaceKeyGetter != nil && w.publicBaseURL != "" {
    chain := []uuid.NullUUID{
        {UUID: email.WorkspaceID, Valid: true},
        // tenant fallback chain if applicable; reuse existing helper
    }
    tt, err := w.templateTypeStore.GetTypeBySlug(ctx, email.TemplateTypeSlug, chain)
    if err == nil && tt != nil && tt.IsBulk {
        key, kerr := w.workspaceKeyGetter.GetUnsubscribeSigningKey(ctx, email.WorkspaceID)
        if kerr != nil {
            slog.Warn("send_worker: failed to load unsubscribe signing key; skipping headers",
                append(emailLogAttrs(email), "error", kerr)...)
        } else {
            now := time.Now().UTC()
            payload := unsubscribe.Payload{
                Version:          1,
                WorkspaceID:      email.WorkspaceID,
                TemplateTypeSlug: tt.Slug,
                TemplateTypeName: tt.Name,
                Email:            domain.CanonicalRecipientAddress(email.RecipientEmail),
                SourceEmailID:    email.ID,
                IssuedAt:         now,
                ExpiresAt:        now.AddDate(1, 0, 0),
            }
            tok, gerr := unsubscribe.Generate(payload, key)
            if gerr != nil {
                slog.Warn("send_worker: failed to generate unsubscribe token; skipping headers",
                    append(emailLogAttrs(email), "error", gerr)...)
            } else {
                base := strings.TrimRight(w.publicBaseURL, "/")
                oneClickURL := base + "/api/v1/u/" + tok
                landingURL  := base + "/u/" + tok
                prefsURL    := base + "/u/" + tok + "/preferences"

                extraHeaders["List-Unsubscribe"] = "<" + oneClickURL + ">"
                extraHeaders["List-Unsubscribe-Post"] = "List-Unsubscribe=One-Click"
                wsName := ""
                if ws, werr := w.workspaceLookup.GetByID(ctx, email.WorkspaceID); werr == nil && ws != nil {
                    wsName = ws.Name
                }
                systemVars = map[string]string{
                    "unsubscribe_url": landingURL,
                    "preferences_url": prefsURL,
                    "workspace_name":  wsName,
                }
            }
        }
    }
}
```

Then **before** step 5 (Render MJML body), if `systemVars != nil`, replace the existing `Render` call with `RenderWithSystem`:

```go
renderedBody, err := w.renderer.RenderWithSystem(payload.BodyMJML, payload.InjectorsSnapshot, payload.VariablesSnapshot, systemVars)
```

(Re-order steps so token generation and `is_bulk` lookup happen *before* render: pull the lookup up to right after step 4 "Load cold payload".)

In step 8 (build outgoing email), merge `extraHeaders` into the headers map:

```go
hdrs := map[string]string{"X-Senda-Tracking-ID": email.TrackingID}
for k, v := range extraHeaders {
    hdrs[k] = v
}
outgoing := &port.OutgoingEmail{
    From: ..., To: ..., Subject: ..., BodyHTML: bodyHTML,
    TrackingID: email.TrackingID,
    Headers:    hdrs,
}
```

Add the import `"github.com/rendis/senda/internal/unsubscribe"` and `"strings"` if missing.

- [ ] **Step 6: Add unit test for the worker integration**

Path: `internal/adapter/river/send_worker_test.go` — add test (build tag may apply; match existing convention):

```go
func TestSendWorker_InjectsListUnsubscribeForBulkTemplate(t *testing.T) {
    // Construct fake stores: template_type with IsBulk=true, workspace key, fake sender that captures Headers.
    // Run worker.Work on a synthetic email row.
    // Assert outgoing.Headers["List-Unsubscribe"] starts with "<https://" + cfg.Tracking.BaseURL + "/api/v1/u/"
    // Assert outgoing.Headers["List-Unsubscribe-Post"] == "List-Unsubscribe=One-Click"
    // ... (full table-driven assertions; mirror existing patterns)
}

func TestSendWorker_SkipsListUnsubscribeForTransactionalTemplate(t *testing.T) {
    // Same setup but IsBulk=false. Assert no List-Unsubscribe* keys present.
}
```

(Adapt to existing fakes / mock patterns in the worker test file.)

- [ ] **Step 7: Run all worker tests**

Run: `go test ./internal/adapter/river/... -run SendWorker -v -race`
Expected: PASS.

- [ ] **Step 8: Run full test suite**

Run: `make test`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/adapter/river/send_worker.go internal/adapter/river/send_worker_test.go \
        internal/service/send.go internal/app/bootstrap.go
git commit -m "feat(send): inject List-Unsubscribe headers and system vars for bulk templates"
```

---

## Task 10: UnsubscribeService

**Files:**
- Create: `internal/service/unsubscribe.go`
- Create: `internal/service/unsubscribe_test.go`

The service exposes the operations the HTTP layer will call:
- `GetContext(ctx, token) (*UnsubscribeContext, error)` — verify token, load workspace name, current state.
- `OneClickOptOut(ctx, token) error` — verify token, write `template_type_subscription` row.
- `OptOutAll(ctx, token) error` — verify token, write `suppression_workspace` with reason=`unsubscribe`.
- `GetPreferences(ctx, token) (*PreferencesView, error)` — verify token, list received template_types in last 12 months + state.
- `UpdatePreferences(ctx, token, changes) error` — verify token, upsert each change.
- `Resubscribe(ctx, token) error` — un-do opt-out-all (sets `removed_at`).

- [ ] **Step 1: Write failing tests**

Path: `internal/service/unsubscribe_test.go`

```go
package service

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/rendis/senda/internal/domain"
    "github.com/rendis/senda/internal/unsubscribe"
)

// Define minimal manual fakes for the dependencies (no mock framework per CLAUDE.md):
//   - workspaceLookup     : returns workspace name + signing key
//   - templateTypeLookup  : FindTypeBySlugInScope
//   - suppressionWriter   : Add (creates suppression_workspace row)
//   - ttsWriter           : Upsert
//   - emailHistoryReader  : DistinctTemplateTypesForRecipient

type fakeWsLookup struct {
    key  []byte
    name string
}
func (f *fakeWsLookup) GetUnsubscribeSigningKey(ctx context.Context, _ uuid.UUID) ([]byte, error) {
    return f.key, nil
}
func (f *fakeWsLookup) GetByID(ctx context.Context, _ uuid.UUID) (*domain.Workspace, error) {
    return &domain.Workspace{ID: uuid.New(), Name: f.name}, nil
}

// ... similarly minimal fakes for the others ...

func TestUnsubscribeService_OneClickOptOut_WritesRow(t *testing.T) {
    ctx := context.Background()
    key := bytesOf(32, 0x42)
    ws := &fakeWsLookup{key: key, name: "Acme Corp"}
    tt := &fakeTtLookup{tt: &domain.TemplateType{ID: uuid.New(), Slug: "newsletter", Name: "Newsletter"}}
    tts := &fakeTtsWriter{}
    svc := NewUnsubscribeService(ws, tt, &fakeSupWriter{}, tts, &fakeEmailHistory{})

    now := time.Now().UTC()
    p := unsubscribe.Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "newsletter", TemplateTypeName: "Newsletter",
        Email: "j@x.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := unsubscribe.Generate(p, key)
    if err := svc.OneClickOptOut(ctx, tok); err != nil {
        t.Fatalf("OneClickOptOut: %v", err)
    }
    if len(tts.upserts) != 1 {
        t.Fatalf("expected 1 upsert, got %d", len(tts.upserts))
    }
    if tts.upserts[0].Subscribed != false || tts.upserts[0].Email != "j@x.com" {
        t.Fatalf("unexpected upsert: %+v", tts.upserts[0])
    }
}

func TestUnsubscribeService_OneClickOptOut_RejectsInvalidToken(t *testing.T) {
    // token signed with a different key
    ctx := context.Background()
    keyA := bytesOf(32, 0x11)
    keyB := bytesOf(32, 0x22)
    ws := &fakeWsLookup{key: keyA, name: "X"}
    tt := &fakeTtLookup{tt: &domain.TemplateType{ID: uuid.New(), Slug: "x", Name: "X"}}
    tts := &fakeTtsWriter{}
    svc := NewUnsubscribeService(ws, tt, &fakeSupWriter{}, tts, &fakeEmailHistory{})

    now := time.Now().UTC()
    p := unsubscribe.Payload{
        Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
        Email: "j@x.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
    }
    tok, _ := unsubscribe.Generate(p, keyB) // wrong key
    err := svc.OneClickOptOut(ctx, tok)
    if err == nil { t.Fatal("expected error on bad token") }
    if len(tts.upserts) != 0 { t.Fatal("must not write on bad token") }
}

func TestUnsubscribeService_GetPreferences_ReturnsHistoricalTypes(t *testing.T) {
    // ... fake history returns 3 template_type slugs received in last 12 months
    // ... fake tts says one is opted-out
    // ... assert returned view has 3 entries with correct subscription states
}

func TestUnsubscribeService_OptOutAll_WritesWorkspaceSuppression(t *testing.T) {
    // ... assert fakeSupWriter.added has SuppressionUnsubscribe reason
}

func TestUnsubscribeService_Resubscribe_UndoesOptOutAll(t *testing.T) {
    // ... assert fakeSupWriter.removed call with workspace + email
}

func bytesOf(n int, b byte) []byte {
    out := make([]byte, n)
    for i := range out { out[i] = b }
    return out
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestUnsubscribeService -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement the service**

Path: `internal/service/unsubscribe.go`

```go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rendis/senda/internal/domain"
    "github.com/rendis/senda/internal/unsubscribe"
)

type UnsubscribeContext struct {
    WorkspaceName    string
    TemplateTypeSlug string
    TemplateTypeName string
    Email            string
    OptedOutOfType   bool
    OptedOutOfAll    bool
}

type PreferencesEntry struct {
    TemplateTypeSlug string
    TemplateTypeName string
    Description      *string
    Subscribed       bool
    LastReceivedAt   time.Time
}

type PreferencesView struct {
    WorkspaceName string
    Email         string
    OptedOutOfAll bool
    Entries       []PreferencesEntry
}

type PreferenceChange struct {
    TemplateTypeSlug string
    Subscribed       bool
}

type workspaceLookup interface {
    GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error)
    GetByID(ctx context.Context, workspaceID uuid.UUID) (*domain.Workspace, error)
}

type templateTypeLookup interface {
    FindTypeBySlugInScope(ctx context.Context, slug string, workspaceID *uuid.UUID) (*domain.TemplateType, error)
}

type suppressionWorkspaceWriter interface {
    Add(ctx context.Context, sup *domain.SuppressionWorkspace) error
    Remove(ctx context.Context, workspaceID uuid.UUID, email string, reason string) error
    GetActive(ctx context.Context, workspaceID uuid.UUID, email string) (*domain.SuppressionWorkspace, error)
}

type ttsWriter interface {
    Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error
    GetState(ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string) (*domain.TemplateTypeSubscription, error)
    ListOptOutsForRecipient(ctx context.Context, workspaceID uuid.UUID, email string) ([]*domain.TemplateTypeSubscription, error)
}

type emailHistoryReader interface {
    DistinctTemplateTypesForRecipient(ctx context.Context, workspaceID uuid.UUID, email string, since time.Time) ([]EmailHistoryType, error)
}

type EmailHistoryType struct {
    Slug          string
    LastSentAt    time.Time
}

type UnsubscribeService struct {
    ws       workspaceLookup
    tt       templateTypeLookup
    supWS    suppressionWorkspaceWriter
    tts      ttsWriter
    history  emailHistoryReader
    now      func() time.Time
}

func NewUnsubscribeService(ws workspaceLookup, tt templateTypeLookup, supWS suppressionWorkspaceWriter, tts ttsWriter, history emailHistoryReader) *UnsubscribeService {
    return &UnsubscribeService{ws: ws, tt: tt, supWS: supWS, tts: tts, history: history, now: func() time.Time { return time.Now().UTC() }}
}

var ErrInvalidToken = errors.New("unsubscribe: invalid token")

func (s *UnsubscribeService) verify(ctx context.Context, token string) (unsubscribe.Payload, []byte, error) {
    // We need the workspace's key to verify, but the token contains the workspace ID.
    // Decode the payload (without verifying yet) to read ws, then load the key, then verify.
    parts := splitTokenParts(token) // helper that reuses base64 decoding
    if parts == nil {
        return unsubscribe.Payload{}, nil, ErrInvalidToken
    }
    wsID, ok := peekWorkspaceID(parts.payload) // reads ws field via json.Decoder without HMAC check
    if !ok {
        return unsubscribe.Payload{}, nil, ErrInvalidToken
    }
    key, err := s.ws.GetUnsubscribeSigningKey(ctx, wsID)
    if err != nil {
        return unsubscribe.Payload{}, nil, fmt.Errorf("%w: workspace lookup: %v", ErrInvalidToken, err)
    }
    p, err := unsubscribe.Verify(token, key, s.now())
    if err != nil {
        return unsubscribe.Payload{}, nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
    }
    return p, key, nil
}

func (s *UnsubscribeService) GetContext(ctx context.Context, token string) (*UnsubscribeContext, error) {
    p, _, err := s.verify(ctx, token)
    if err != nil { return nil, err }
    ws, err := s.ws.GetByID(ctx, p.WorkspaceID)
    if err != nil { return nil, fmt.Errorf("workspace name: %w", err) }
    tt, err := s.tt.FindTypeBySlugInScope(ctx, p.TemplateTypeSlug, &p.WorkspaceID)
    if err != nil && !errors.Is(err, domain.ErrNotFound) { return nil, err }
    var typeID uuid.UUID
    if tt != nil { typeID = tt.ID }
    optedOutType := false
    if tt != nil {
        sub, err := s.tts.GetState(ctx, p.WorkspaceID, typeID, p.Email)
        if err != nil && !errors.Is(err, domain.ErrNotFound) { return nil, err }
        if sub != nil && !sub.Subscribed { optedOutType = true }
    }
    sup, err := s.supWS.GetActive(ctx, p.WorkspaceID, p.Email)
    if err != nil && !errors.Is(err, domain.ErrNotFound) { return nil, err }
    return &UnsubscribeContext{
        WorkspaceName:    ws.Name,
        TemplateTypeSlug: p.TemplateTypeSlug,
        TemplateTypeName: p.TemplateTypeName,
        Email:            p.Email,
        OptedOutOfType:   optedOutType,
        OptedOutOfAll:    sup != nil && sup.Reason == domain.SuppressionUnsubscribe,
    }, nil
}

func (s *UnsubscribeService) OneClickOptOut(ctx context.Context, token string) error {
    p, _, err := s.verify(ctx, token)
    if err != nil { return err }
    tt, err := s.tt.FindTypeBySlugInScope(ctx, p.TemplateTypeSlug, &p.WorkspaceID)
    if err != nil { return fmt.Errorf("resolve type: %w", err) }
    return s.tts.Upsert(ctx, &domain.TemplateTypeSubscription{
        ID:             uuid.Must(uuid.NewV7()),
        WorkspaceID:    p.WorkspaceID,
        TemplateTypeID: tt.ID,
        Email:          p.Email,
        Subscribed:     false,
        Source:         domain.SubscriptionSourceRecipientOptout,
        SourceEmailID:  &p.SourceEmailID,
    })
}

func (s *UnsubscribeService) OptOutAll(ctx context.Context, token string) error {
    p, _, err := s.verify(ctx, token)
    if err != nil { return err }
    return s.supWS.Add(ctx, &domain.SuppressionWorkspace{
        ID:            uuid.Must(uuid.NewV7()),
        WorkspaceID:   p.WorkspaceID,
        Email:         p.Email,
        Reason:        domain.SuppressionUnsubscribe,
        SourceEmailID: &p.SourceEmailID,
    })
}

func (s *UnsubscribeService) GetPreferences(ctx context.Context, token string) (*PreferencesView, error) {
    p, _, err := s.verify(ctx, token)
    if err != nil { return nil, err }
    ws, err := s.ws.GetByID(ctx, p.WorkspaceID)
    if err != nil { return nil, err }

    since := s.now().AddDate(-1, 0, 0)
    history, err := s.history.DistinctTemplateTypesForRecipient(ctx, p.WorkspaceID, p.Email, since)
    if err != nil { return nil, err }

    sup, err := s.supWS.GetActive(ctx, p.WorkspaceID, p.Email)
    if err != nil && !errors.Is(err, domain.ErrNotFound) { return nil, err }
    optedAll := sup != nil && sup.Reason == domain.SuppressionUnsubscribe

    entries := make([]PreferencesEntry, 0, len(history))
    for _, h := range history {
        tt, err := s.tt.FindTypeBySlugInScope(ctx, h.Slug, &p.WorkspaceID)
        if err != nil || tt == nil { continue }
        // Default = subscribed=true when no row exists (ErrNotFound).
        sub, err := s.tts.GetState(ctx, p.WorkspaceID, tt.ID, p.Email)
        subscribed := true
        if err == nil && sub != nil {
            subscribed = sub.Subscribed
        } else if err != nil && !errors.Is(err, domain.ErrNotFound) {
            return nil, err
        }
        entries = append(entries, PreferencesEntry{
            TemplateTypeSlug: tt.Slug,
            TemplateTypeName: tt.Name,
            Description:      tt.Description,
            Subscribed:       subscribed,
            LastReceivedAt:   h.LastSentAt,
        })
    }
    return &PreferencesView{
        WorkspaceName: ws.Name,
        Email:         p.Email,
        OptedOutOfAll: optedAll,
        Entries:       entries,
    }, nil
}

func (s *UnsubscribeService) UpdatePreferences(ctx context.Context, token string, changes []PreferenceChange) error {
    p, _, err := s.verify(ctx, token)
    if err != nil { return err }
    for _, c := range changes {
        tt, err := s.tt.FindTypeBySlugInScope(ctx, c.TemplateTypeSlug, &p.WorkspaceID)
        if err != nil { return fmt.Errorf("resolve type %s: %w", c.TemplateTypeSlug, err) }
        src := domain.SubscriptionSourceRecipientOptin
        if !c.Subscribed { src = domain.SubscriptionSourceRecipientOptout }
        if err := s.tts.Upsert(ctx, &domain.TemplateTypeSubscription{
            ID:             uuid.Must(uuid.NewV7()),
            WorkspaceID:    p.WorkspaceID,
            TemplateTypeID: tt.ID,
            Email:          p.Email,
            Subscribed:     c.Subscribed,
            Source:         src,
            SourceEmailID:  &p.SourceEmailID,
        }); err != nil {
            return err
        }
    }
    return nil
}

func (s *UnsubscribeService) Resubscribe(ctx context.Context, token string) error {
    p, _, err := s.verify(ctx, token)
    if err != nil { return err }
    return s.supWS.Remove(ctx, p.WorkspaceID, p.Email, "recipient_resubscribe")
}
```

> Implementation note: `splitTokenParts` and `peekWorkspaceID` are small helpers in this file (or `internal/unsubscribe/`) that decode the base64 payload only to read the `ws` field — used to look up the key before HMAC verification. The actual verification still happens via `unsubscribe.Verify`. Add these as part of this task.

Add the helpers in `internal/unsubscribe/peek.go`:

```go
package unsubscribe

import (
    "encoding/base64"
    "encoding/json"
    "strings"

    "github.com/google/uuid"
)

// PeekWorkspaceID extracts the workspace UUID from a token without verifying
// its HMAC signature. The caller MUST then call Verify with the workspace's
// signing key. PeekWorkspaceID returns ok=false if the token is malformed.
func PeekWorkspaceID(token string) (uuid.UUID, bool) {
    parts := strings.SplitN(token, ".", 2)
    if len(parts) != 2 {
        return uuid.Nil, false
    }
    body, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil {
        return uuid.Nil, false
    }
    var w struct {
        WS uuid.UUID `json:"ws"`
    }
    if err := json.Unmarshal(body, &w); err != nil {
        return uuid.Nil, false
    }
    return w.WS, true
}
```

Use `unsubscribe.PeekWorkspaceID(token)` in the service's `verify` helper instead of the placeholder.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestUnsubscribeService -v -race`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/unsubscribe.go internal/service/unsubscribe_test.go internal/unsubscribe/peek.go
git commit -m "feat(service): UnsubscribeService with token verify, opt-out, preferences, resubscribe"
```

---

## Task 11: Email history reader for preference center

**Files:**
- Modify: `internal/port/store.go` (or `email_store.go` port)
- Modify: `internal/adapter/postgres/email_store.go`
- Modify: `internal/adapter/postgres/email_store_test.go`

The preference center needs to know which template types a recipient has received in the last 12 months.

- [ ] **Step 1: Write failing test**

Append to `internal/adapter/postgres/email_store_test.go`:

```go
//go:build integration

func TestEmailStore_DistinctTemplateTypesForRecipient(t *testing.T) {
    ctx := context.Background()
    pool, cleanup := newPgxPool(t)
    defer cleanup()
    wsID := seedWorkspace(t, ctx, pool)

    // Insert 3 emails: 2 distinct template types within 12 months, 1 expired (>12 months)
    _ = insertEmail(t, ctx, pool, wsID, "j@x.com", "newsletter", time.Now().Add(-30*24*time.Hour))
    _ = insertEmail(t, ctx, pool, wsID, "j@x.com", "alerts",     time.Now().Add(-7*24*time.Hour))
    _ = insertEmail(t, ctx, pool, wsID, "j@x.com", "promo",      time.Now().Add(-400*24*time.Hour)) // out of window

    store := NewEmailStore(pool)
    res, err := store.DistinctTemplateTypesForRecipient(ctx, wsID, "j@x.com", time.Now().AddDate(-1, 0, 0))
    if err != nil { t.Fatalf("err: %v", err) }
    if len(res) != 2 {
        t.Fatalf("expected 2 distinct types in window, got %d", len(res))
    }
    slugs := []string{res[0].Slug, res[1].Slug}
    sort.Strings(slugs)
    if slugs[0] != "alerts" || slugs[1] != "newsletter" {
        t.Fatalf("unexpected slugs: %v", slugs)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestEmailStore_DistinctTemplateTypes -v`
Expected: FAIL — method undefined.

- [ ] **Step 3: Implement method on EmailStore**

Add to `internal/adapter/postgres/email_store.go`:

```go
type EmailHistoryType = service.EmailHistoryType // alias to avoid duplicate type

func (s *EmailStore) DistinctTemplateTypesForRecipient(
    ctx context.Context, workspaceID uuid.UUID, email string, since time.Time,
) ([]service.EmailHistoryType, error) {
    rows, err := s.db.Query(ctx, `
        SELECT template_type_slug, MAX(created_at) AS last_sent
        FROM emails
        WHERE workspace_id = $1
          AND recipient_email = $2
          AND created_at >= $3
        GROUP BY template_type_slug
        ORDER BY last_sent DESC
    `, workspaceID, email, since)
    if err != nil { return nil, fmt.Errorf("emails: distinct types: %w", err) }
    defer rows.Close()
    out := []service.EmailHistoryType{}
    for rows.Next() {
        var t service.EmailHistoryType
        if err := rows.Scan(&t.Slug, &t.LastSentAt); err != nil {
            return nil, fmt.Errorf("scan: %w", err)
        }
        out = append(out, t)
    }
    return out, rows.Err()
}
```

(If circular import warns between `service` and `postgres`, define the struct in `internal/port` and use it in both.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags=integration ./internal/adapter/postgres/ -run TestEmailStore_DistinctTemplateTypes -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/port/store.go internal/adapter/postgres/email_store.go internal/adapter/postgres/email_store_test.go
git commit -m "feat(emails): DistinctTemplateTypesForRecipient for preference center"
```

---

## Task 12: HTTP layer — request/response/handler/routes

**Files:**
- Create: `internal/http/request/unsubscribe.go`
- Create: `internal/http/response/unsubscribe.go`
- Create: `internal/http/handler/unsubscribe.go`
- Create: `internal/http/handler/unsubscribe_test.go`
- Modify: `internal/http/server.go`
- Modify: `internal/http/routes_public.go`
- Modify: `internal/openapi/openapi.go`
- Modify: `internal/app/bootstrap.go`

- [ ] **Step 1: Define request DTOs**

Path: `internal/http/request/unsubscribe.go`

```go
package request

type UpdatePreferencesRequest struct {
    Changes []PreferenceChange `json:"changes"`
}

type PreferenceChange struct {
    TemplateTypeSlug string `json:"template_type_slug"`
    Subscribed       bool   `json:"subscribed"`
}
```

- [ ] **Step 2: Define response DTOs**

Path: `internal/http/response/unsubscribe.go`

```go
package response

import "time"

type UnsubscribeContextResponse struct {
    WorkspaceName    string `json:"workspace_name"`
    TemplateTypeSlug string `json:"template_type_slug"`
    TemplateTypeName string `json:"template_type_name"`
    Email            string `json:"email"`
    OptedOutOfType   bool   `json:"opted_out_of_type"`
    OptedOutOfAll    bool   `json:"opted_out_of_all"`
}

type PreferencesEntryResponse struct {
    TemplateTypeSlug string    `json:"template_type_slug"`
    TemplateTypeName string    `json:"template_type_name"`
    Description      *string   `json:"description,omitempty"`
    Subscribed       bool      `json:"subscribed"`
    LastReceivedAt   time.Time `json:"last_received_at"`
}

type PreferencesViewResponse struct {
    WorkspaceName string                     `json:"workspace_name"`
    Email         string                     `json:"email"`
    OptedOutOfAll bool                       `json:"opted_out_of_all"`
    Entries       []PreferencesEntryResponse `json:"entries"`
}
```

- [ ] **Step 3: Write failing handler test**

Path: `internal/http/handler/unsubscribe_test.go`

```go
package handler_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/labstack/echo/v5"
    "github.com/rendis/senda/internal/http/handler"
)

type stubUnsubscribeService struct {
    oneClickCalled bool
    oneClickToken  string
    err            error
}

func (s *stubUnsubscribeService) OneClickOptOut(ctx context.Context, token string) error {
    s.oneClickCalled = true
    s.oneClickToken = token
    return s.err
}
// ... implement the other UnsubscribeService methods to satisfy the interface

func TestHandler_OneClickOptOut_Success(t *testing.T) {
    e := echo.New()
    svc := &stubUnsubscribeService{}
    h := handler.NewUnsubscribeHandler(svc)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/u/abc.def", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("token")
    c.SetParamValues("abc.def")
    if err := h.OneClickOptOut(c); err != nil { t.Fatalf("handler: %v", err) }
    if rec.Code != 200 { t.Fatalf("status: %d", rec.Code) }
    if svc.oneClickToken != "abc.def" { t.Fatalf("token: %s", svc.oneClickToken) }
}

func TestHandler_OneClickOptOut_InvalidToken_Returns404(t *testing.T) {
    e := echo.New()
    svc := &stubUnsubscribeService{err: service.ErrInvalidToken}
    h := handler.NewUnsubscribeHandler(svc)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/u/bad", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("token")
    c.SetParamValues("bad")
    _ = h.OneClickOptOut(c)
    if rec.Code != 404 { t.Fatalf("status: %d, want 404", rec.Code) }
}

func TestHandler_GetContext_Success(t *testing.T) {
    // ... set up stub.GetContext to return a fixed UnsubscribeContext
    // ... assert response JSON shape matches UnsubscribeContextResponse
}

func TestHandler_GetPreferences_Success(t *testing.T) {
    // ... assert response JSON has entries with correct shape
}

func TestHandler_UpdatePreferences_PersistsChanges(t *testing.T) {
    // ... POST {"changes":[...]}, assert stub.UpdatePreferencesCalled with correct slice
}

func TestHandler_OptOutAll_Success(t *testing.T) {
    // ... POST /api/v1/u/:token/all, assert stub called
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/http/handler/ -run TestHandler_OneClick -v`
Expected: FAIL — `NewUnsubscribeHandler` undefined.

- [ ] **Step 5: Implement the handler**

Path: `internal/http/handler/unsubscribe.go`

```go
package handler

import (
    "context"
    "errors"
    "net/http"

    "github.com/labstack/echo/v5"
    "github.com/rendis/senda/internal/http/request"
    "github.com/rendis/senda/internal/http/response"
    "github.com/rendis/senda/internal/service"
)

type unsubscribeService interface {
    GetContext(ctx context.Context, token string) (*service.UnsubscribeContext, error)
    OneClickOptOut(ctx context.Context, token string) error
    OptOutAll(ctx context.Context, token string) error
    GetPreferences(ctx context.Context, token string) (*service.PreferencesView, error)
    UpdatePreferences(ctx context.Context, token string, changes []service.PreferenceChange) error
    Resubscribe(ctx context.Context, token string) error
}

type UnsubscribeHandler struct {
    svc unsubscribeService
}

func NewUnsubscribeHandler(svc unsubscribeService) *UnsubscribeHandler {
    return &UnsubscribeHandler{svc: svc}
}

func (h *UnsubscribeHandler) GetContext(c *echo.Context) error {
    token := c.PathParam("token")
    ctx, err := h.svc.GetContext(c.Request().Context(), token)
    if err != nil { return mapUnsubscribeError(c, err) }
    return c.JSON(http.StatusOK, response.UnsubscribeContextResponse{
        WorkspaceName:    ctx.WorkspaceName,
        TemplateTypeSlug: ctx.TemplateTypeSlug,
        TemplateTypeName: ctx.TemplateTypeName,
        Email:            ctx.Email,
        OptedOutOfType:   ctx.OptedOutOfType,
        OptedOutOfAll:    ctx.OptedOutOfAll,
    })
}

func (h *UnsubscribeHandler) OneClickOptOut(c *echo.Context) error {
    token := c.PathParam("token")
    if err := h.svc.OneClickOptOut(c.Request().Context(), token); err != nil {
        return mapUnsubscribeError(c, err)
    }
    return c.NoContent(http.StatusOK)
}

func (h *UnsubscribeHandler) OptOutAll(c *echo.Context) error {
    token := c.PathParam("token")
    if err := h.svc.OptOutAll(c.Request().Context(), token); err != nil {
        return mapUnsubscribeError(c, err)
    }
    return c.NoContent(http.StatusOK)
}

func (h *UnsubscribeHandler) GetPreferences(c *echo.Context) error {
    token := c.PathParam("token")
    view, err := h.svc.GetPreferences(c.Request().Context(), token)
    if err != nil { return mapUnsubscribeError(c, err) }
    out := response.PreferencesViewResponse{
        WorkspaceName: view.WorkspaceName,
        Email:         view.Email,
        OptedOutOfAll: view.OptedOutOfAll,
        Entries:       make([]response.PreferencesEntryResponse, 0, len(view.Entries)),
    }
    for _, e := range view.Entries {
        out.Entries = append(out.Entries, response.PreferencesEntryResponse{
            TemplateTypeSlug: e.TemplateTypeSlug,
            TemplateTypeName: e.TemplateTypeName,
            Description:      e.Description,
            Subscribed:       e.Subscribed,
            LastReceivedAt:   e.LastReceivedAt,
        })
    }
    return c.JSON(http.StatusOK, out)
}

func (h *UnsubscribeHandler) UpdatePreferences(c *echo.Context) error {
    token := c.PathParam("token")
    var req request.UpdatePreferencesRequest
    if err := c.Bind(&req); err != nil {
        return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid body")
    }
    changes := make([]service.PreferenceChange, 0, len(req.Changes))
    for _, ch := range req.Changes {
        changes = append(changes, service.PreferenceChange{
            TemplateTypeSlug: ch.TemplateTypeSlug,
            Subscribed:       ch.Subscribed,
        })
    }
    if err := h.svc.UpdatePreferences(c.Request().Context(), token, changes); err != nil {
        return mapUnsubscribeError(c, err)
    }
    return c.NoContent(http.StatusOK)
}

func (h *UnsubscribeHandler) Resubscribe(c *echo.Context) error {
    token := c.PathParam("token")
    if err := h.svc.Resubscribe(c.Request().Context(), token); err != nil {
        return mapUnsubscribeError(c, err)
    }
    return c.NoContent(http.StatusOK)
}

func mapUnsubscribeError(c *echo.Context, err error) error {
    if errors.Is(err, service.ErrInvalidToken) {
        return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "invalid or expired link")
    }
    return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected error")
}
```

- [ ] **Step 6: Wire to server**

Edit `internal/http/server.go`:

```go
type Server struct {
    // ... existing fields ...
    unsubscribeHandler *handler.UnsubscribeHandler
}

func WithUnsubscribeHandler(h *handler.UnsubscribeHandler) ServerOption {
    return func(s *Server) { s.unsubscribeHandler = h }
}
```

- [ ] **Step 7: Register public routes**

Edit `internal/http/routes_public.go`. Inside `registerPublicRoutes()`, after the existing public endpoints:

```go
if s.unsubscribeHandler != nil {
    s.echo.GET ("/api/v1/u/:token",             s.unsubscribeHandler.GetContext)
    s.echo.POST("/api/v1/u/:token",             s.unsubscribeHandler.OneClickOptOut)
    s.echo.POST("/api/v1/u/:token/all",         s.unsubscribeHandler.OptOutAll)
    s.echo.POST("/api/v1/u/:token/resubscribe", s.unsubscribeHandler.Resubscribe)
    s.echo.GET ("/api/v1/u/:token/preferences", s.unsubscribeHandler.GetPreferences)
    s.echo.POST("/api/v1/u/:token/preferences", s.unsubscribeHandler.UpdatePreferences)
}
```

- [ ] **Step 8: Update OpenAPI generator**

Edit `internal/openapi/openapi.go`. In the `routeRequestType()` switch (around line 421), add:

```go
case strings.HasSuffix(path, "/preferences") && method == http.MethodPost:
    return "request.UpdatePreferencesRequest", true
```

Verify routes appear in the generated docs by running `make generate-openapi` (or whatever target generates the docs).

- [ ] **Step 9: Wire UnsubscribeService in bootstrap**

Edit `internal/app/bootstrap.go`. After services are constructed, add:

```go
unsubSvc := service.NewUnsubscribeService(
    repos.workspaceStore,
    repos.templateTypeStore,
    repos.suppressionStore,
    repos.templateTypeSubscriptionStore,
    repos.emailStore, // implements DistinctTemplateTypesForRecipient
)
unsubH := handler.NewUnsubscribeHandler(unsubSvc)

serverOpts = append(serverOpts, http_.WithUnsubscribeHandler(unsubH))
```

(Adjust import alias `http_` to match the existing convention.)

- [ ] **Step 10: Run handler tests**

Run: `go test ./internal/http/handler/ -run TestHandler -v -race`
Expected: all PASS.

- [ ] **Step 11: Build the binary**

Run: `make build`
Expected: success.

- [ ] **Step 12: Commit**

```bash
git add internal/http/ internal/openapi/openapi.go internal/app/bootstrap.go
git commit -m "feat(http): public unsubscribe routes (GET/POST :token, /all, /preferences, /resubscribe)"
```

---

## Task 13: Frontend — install missing shadcn components

**Files:**
- Create: `web/src/components/ui/checkbox.tsx`
- Create: `web/src/components/ui/radio-group.tsx`
- Create: `web/src/components/ui/alert.tsx`

- [ ] **Step 1: Add the components**

```bash
cd web
pnpm dlx shadcn@latest add checkbox radio-group alert
cd ..
```

- [ ] **Step 2: Verify files were created**

Run: `ls web/src/components/ui/{checkbox,radio-group,alert}.tsx`
Expected: three files listed.

- [ ] **Step 3: Verify nothing broke**

Run: `pnpm --dir web typecheck && pnpm --dir web lint`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ui/checkbox.tsx web/src/components/ui/radio-group.tsx web/src/components/ui/alert.tsx web/package.json web/pnpm-lock.yaml
git commit -m "chore(web): install shadcn checkbox, radio-group, alert"
```

---

## Task 14: Frontend — public API client

**Files:**
- Create: `web/src/lib/unsubscribe-api.ts`

- [ ] **Step 1: Implement the client**

Path: `web/src/lib/unsubscribe-api.ts`

```ts
import ky from "ky";

const api = ky.create({ prefixUrl: "/api/v1" });

export type UnsubscribeContext = {
  workspace_name: string;
  template_type_slug: string;
  template_type_name: string;
  email: string;
  opted_out_of_type: boolean;
  opted_out_of_all: boolean;
};

export type PreferencesEntry = {
  template_type_slug: string;
  template_type_name: string;
  description?: string;
  subscribed: boolean;
  last_received_at: string;
};

export type PreferencesView = {
  workspace_name: string;
  email: string;
  opted_out_of_all: boolean;
  entries: PreferencesEntry[];
};

export async function getContext(token: string): Promise<UnsubscribeContext> {
  return api.get(`u/${encodeURIComponent(token)}`).json<UnsubscribeContext>();
}

export async function optOutThisType(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}`);
}

export async function optOutAll(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/all`);
}

export async function resubscribe(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/resubscribe`);
}

export async function getPreferences(token: string): Promise<PreferencesView> {
  return api.get(`u/${encodeURIComponent(token)}/preferences`).json<PreferencesView>();
}

export async function updatePreferences(
  token: string,
  changes: { template_type_slug: string; subscribed: boolean }[],
): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/preferences`, { json: { changes } });
}
```

- [ ] **Step 2: Verify typecheck**

Run: `pnpm --dir web typecheck`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/unsubscribe-api.ts
git commit -m "feat(web): public unsubscribe API client"
```

---

## Task 15: Frontend — i18n strings

**Files:**
- Modify: `web/messages/en.json`
- Modify: `web/messages/es.json`

- [ ] **Step 1: Add English keys**

Append to `web/messages/en.json` (within the root JSON object):

```json
"unsubscribe": {
  "loading": "Loading…",
  "title_for_event": "You are unsubscribing from {workspace_name}",
  "this_event_label": "Stop receiving \"{template_type_name}\"",
  "all_label": "Stop receiving ALL emails from {workspace_name}",
  "confirm": "Confirm",
  "manage_link": "Manage all my preferences",
  "success_event": "You will no longer receive \"{template_type_name}\".",
  "success_all": "You will no longer receive any emails from {workspace_name}.",
  "preferences_title": "Email preferences for {email}",
  "preferences_subtitle": "Showing types you have received in the last 12 months from {workspace_name}.",
  "save": "Save",
  "saved": "Preferences saved",
  "all_warning": "You opted out of all emails from {workspace_name}.",
  "resubscribe_all": "Resubscribe to all",
  "invalid_token": "This link is invalid or has expired.",
  "last_received": "Last received"
}
```

- [ ] **Step 2: Add Spanish keys**

Append to `web/messages/es.json`:

```json
"unsubscribe": {
  "loading": "Cargando…",
  "title_for_event": "Estás dejando de recibir correos de {workspace_name}",
  "this_event_label": "Dejar de recibir \"{template_type_name}\"",
  "all_label": "Dejar de recibir TODOS los correos de {workspace_name}",
  "confirm": "Confirmar",
  "manage_link": "Gestionar todas mis preferencias",
  "success_event": "Ya no recibirás \"{template_type_name}\".",
  "success_all": "Ya no recibirás ningún correo de {workspace_name}.",
  "preferences_title": "Preferencias de correo para {email}",
  "preferences_subtitle": "Mostrando tipos que recibiste en los últimos 12 meses de {workspace_name}.",
  "save": "Guardar",
  "saved": "Preferencias guardadas",
  "all_warning": "Te diste de baja de todos los correos de {workspace_name}.",
  "resubscribe_all": "Reactivar todos",
  "invalid_token": "Este enlace no es válido o expiró.",
  "last_received": "Último envío"
}
```

- [ ] **Step 3: Commit**

```bash
git add web/messages/en.json web/messages/es.json
git commit -m "feat(web): unsubscribe i18n strings (en + es)"
```

---

## Task 16: Frontend — `/u/[token]` landing page

**Files:**
- Create: `web/src/app/u/[token]/layout.tsx`
- Create: `web/src/app/u/[token]/page.tsx`
- Create: `web/src/components/unsubscribe/unsubscribe-form.tsx`

- [ ] **Step 1: Minimal layout (no dashboard chrome)**

Path: `web/src/app/u/[token]/layout.tsx`

```tsx
import { ReactNode } from "react";

export default function UnsubscribeLayout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">{children}</div>
    </div>
  );
}
```

- [ ] **Step 2: Server component page**

Path: `web/src/app/u/[token]/page.tsx`

```tsx
import { getContext } from "@/lib/unsubscribe-api";
import { UnsubscribeForm } from "@/components/unsubscribe/unsubscribe-form";
import { getTranslations } from "next-intl/server";
import { Alert, AlertDescription } from "@/components/ui/alert";

export const dynamic = "force-dynamic";

export default async function UnsubscribePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const t = await getTranslations("unsubscribe");
  let ctx;
  try {
    ctx = await getContext(token);
  } catch {
    return (
      <Alert variant="destructive" data-testid="invalid-token-alert">
        <AlertDescription>{t("invalid_token")}</AlertDescription>
      </Alert>
    );
  }
  return <UnsubscribeForm token={token} initialContext={ctx} />;
}
```

- [ ] **Step 3: Client component form**

Path: `web/src/components/unsubscribe/unsubscribe-form.tsx`

```tsx
"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { optOutThisType, optOutAll, type UnsubscribeContext } from "@/lib/unsubscribe-api";
import { toast } from "sonner";

type Choice = "this_event" | "all";

export function UnsubscribeForm({
  token,
  initialContext,
}: {
  token: string;
  initialContext: UnsubscribeContext;
}) {
  const t = useTranslations("unsubscribe");
  const [choice, setChoice] = useState<Choice>("this_event");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState<Choice | null>(null);

  async function onConfirm() {
    setSubmitting(true);
    try {
      if (choice === "this_event") {
        await optOutThisType(token);
        toast.success(t("success_event", { template_type_name: initialContext.template_type_name }));
      } else {
        await optOutAll(token);
        toast.success(t("success_all", { workspace_name: initialContext.workspace_name }));
      }
      setDone(choice);
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <Card data-testid="success-card">
        <CardContent className="pt-6">
          <Alert>
            <AlertDescription>
              {done === "this_event"
                ? t("success_event", { template_type_name: initialContext.template_type_name })
                : t("success_all", { workspace_name: initialContext.workspace_name })}
            </AlertDescription>
          </Alert>
          <div className="mt-4">
            <Link href={`/u/${token}/preferences`} className="text-sm underline" data-testid="manage-link">
              {t("manage_link")}
            </Link>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card data-testid="unsubscribe-card">
      <CardHeader>
        <CardTitle>
          {t("title_for_event", { workspace_name: initialContext.workspace_name })}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <RadioGroup
          value={choice}
          onValueChange={(v) => setChoice(v as Choice)}
        >
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="this_event" id="this_event" data-testid="radio-this-event" />
            <Label htmlFor="this_event">
              {t("this_event_label", { template_type_name: initialContext.template_type_name })}
            </Label>
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="all" id="all" data-testid="radio-all" />
            <Label htmlFor="all">
              {t("all_label", { workspace_name: initialContext.workspace_name })}
            </Label>
          </div>
        </RadioGroup>

        <Button onClick={onConfirm} disabled={submitting} data-testid="confirm-button" className="w-full">
          {t("confirm")}
        </Button>

        <Link href={`/u/${token}/preferences`} className="block text-sm underline text-center" data-testid="manage-link">
          {t("manage_link")}
        </Link>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 4: Verify typecheck and lint**

Run: `pnpm --dir web typecheck && pnpm --dir web lint`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/app/u/ web/src/components/unsubscribe/unsubscribe-form.tsx
git commit -m "feat(web): unsubscribe landing page /u/[token]"
```

---

## Task 17: Frontend — `/u/[token]/preferences` page

**Files:**
- Create: `web/src/app/u/[token]/preferences/page.tsx`
- Create: `web/src/components/unsubscribe/preferences-form.tsx`

- [ ] **Step 1: Server component page**

Path: `web/src/app/u/[token]/preferences/page.tsx`

```tsx
import { getPreferences } from "@/lib/unsubscribe-api";
import { PreferencesForm } from "@/components/unsubscribe/preferences-form";
import { getTranslations } from "next-intl/server";
import { Alert, AlertDescription } from "@/components/ui/alert";

export const dynamic = "force-dynamic";

export default async function PreferencesPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const t = await getTranslations("unsubscribe");
  try {
    const view = await getPreferences(token);
    return <PreferencesForm token={token} initialView={view} />;
  } catch {
    return (
      <Alert variant="destructive" data-testid="invalid-token-alert">
        <AlertDescription>{t("invalid_token")}</AlertDescription>
      </Alert>
    );
  }
}
```

- [ ] **Step 2: Client component**

Path: `web/src/components/unsubscribe/preferences-form.tsx`

```tsx
"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { resubscribe, updatePreferences, type PreferencesView } from "@/lib/unsubscribe-api";
import { toast } from "sonner";

export function PreferencesForm({
  token,
  initialView,
}: {
  token: string;
  initialView: PreferencesView;
}) {
  const t = useTranslations("unsubscribe");
  const [view, setView] = useState<PreferencesView>(initialView);
  const [saving, setSaving] = useState(false);

  function toggle(slug: string, next: boolean) {
    setView((v) => ({
      ...v,
      entries: v.entries.map((e) =>
        e.template_type_slug === slug ? { ...e, subscribed: next } : e,
      ),
    }));
  }

  async function onSave() {
    setSaving(true);
    try {
      const changes = view.entries.map((e) => ({
        template_type_slug: e.template_type_slug,
        subscribed: e.subscribed,
      }));
      await updatePreferences(token, changes);
      toast.success(t("saved"));
    } finally {
      setSaving(false);
    }
  }

  async function onResubscribeAll() {
    setSaving(true);
    try {
      await resubscribe(token);
      setView((v) => ({ ...v, opted_out_of_all: false }));
      toast.success(t("saved"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card data-testid="preferences-card">
      <CardHeader>
        <CardTitle>{t("preferences_title", { email: view.email })}</CardTitle>
        <p className="text-sm text-muted-foreground">
          {t("preferences_subtitle", { workspace_name: view.workspace_name })}
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        {view.opted_out_of_all && (
          <Alert variant="destructive" data-testid="opted-out-all-alert">
            <AlertDescription className="flex items-center justify-between">
              <span>{t("all_warning", { workspace_name: view.workspace_name })}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={onResubscribeAll}
                data-testid="resubscribe-all-button"
              >
                {t("resubscribe_all")}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        <div className="space-y-2">
          {view.entries.map((e) => (
            <div
              key={e.template_type_slug}
              className="flex items-start space-x-3 p-2 rounded hover:bg-muted/40"
              data-testid={`pref-row-${e.template_type_slug}`}
            >
              <Checkbox
                id={`pref-${e.template_type_slug}`}
                checked={e.subscribed}
                onCheckedChange={(v) => toggle(e.template_type_slug, Boolean(v))}
                data-testid={`pref-cb-${e.template_type_slug}`}
              />
              <div className="flex-1">
                <label
                  htmlFor={`pref-${e.template_type_slug}`}
                  className="font-medium cursor-pointer"
                >
                  {e.template_type_name}
                </label>
                {e.description && (
                  <p className="text-sm text-muted-foreground">{e.description}</p>
                )}
                <p className="text-xs text-muted-foreground mt-1">
                  {t("last_received")}: {new Date(e.last_received_at).toLocaleDateString()}
                </p>
              </div>
            </div>
          ))}
        </div>

        <Button
          onClick={onSave}
          disabled={saving}
          data-testid="save-button"
          className="w-full"
        >
          {t("save")}
        </Button>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Verify typecheck and lint**

Run: `pnpm --dir web typecheck && pnpm --dir web lint`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/app/u/\[token\]/preferences/ web/src/components/unsubscribe/preferences-form.tsx
git commit -m "feat(web): preference center /u/[token]/preferences"
```

---

## Task 18: Backend E2E test — Mailpit + headers + flow (build tag `e2e`)

**Files:**
- Create: `test/e2e/unsubscribe_e2e_test.go`

This test uses the existing harness (Testcontainers Postgres + Mailpit + App). It does NOT require the frontend.

- [ ] **Step 1: Write the test**

Path: `test/e2e/unsubscribe_e2e_test.go`

```go
//go:build e2e

package e2e

import (
    "fmt"
    "net/http"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestUnsubscribe_BulkTemplate_HeadersAndOneClickPipeline(t *testing.T) {
    EnsureSetup(t)
    cli := NewTestClient(t)
    mailpit := NewMailpitClient(t)
    mailpit.ClearMessages()

    // 1. Create a template_type with is_bulk=true.
    typeSlug := fmt.Sprintf("unsub-bulk-%d", time.Now().UnixNano())
    cli.CreateTemplateType(t, TemplateTypeInput{
        Slug:    typeSlug,
        Name:    "Unsubscribe Bulk Test",
        IsBulk:  true,
    })

    // 2. Create a template that uses the system variables in its MJML.
    mjml := `<mjml><mj-body><mj-section><mj-column>
        <mj-text>Hi {{ event.name }}</mj-text>
        <mj-text><a href="{{ system.unsubscribe_url }}">unsub</a></mj-text>
        <mj-text><a href="{{ system.preferences_url }}">prefs</a></mj-text>
    </mj-column></mj-section></mj-body></mjml>`
    cli.CreateAndPublishTemplate(t, typeSlug, "Unsub Bulk", mjml)

    // 3. Send.
    recipient := fmt.Sprintf("recipient+%d@e2e.test", time.Now().UnixNano())
    cli.SendEmail(t, SendEmailInput{
        TemplateTypeSlug: typeSlug,
        TemplateSlug:     "unsub-bulk",
        Recipient:        recipient,
        Variables:        map[string]any{"name": "E2E"},
    })

    // 4. Wait for delivery.
    var msgID string
    require.Eventually(t, func() bool {
        msgs := mailpit.GetMessages()
        for _, m := range msgs {
            if strings.Contains(strings.Join(m.To, ","), recipient) {
                msgID = m.ID
                return true
            }
        }
        return false
    }, 30*time.Second, 250*time.Millisecond, "email never arrived")

    // 5. Read raw message and assert headers.
    msg := mailpit.GetMessage(msgID)
    var listUnsub, listUnsubPost string
    for _, h := range msg.Headers {
        if strings.EqualFold(h.Name, "List-Unsubscribe") {
            listUnsub = h.Value
        }
        if strings.EqualFold(h.Name, "List-Unsubscribe-Post") {
            listUnsubPost = h.Value
        }
    }
    require.NotEmpty(t, listUnsub, "List-Unsubscribe header missing")
    require.Equal(t, "List-Unsubscribe=One-Click", listUnsubPost, "List-Unsubscribe-Post must be one-click")
    require.True(t, strings.HasPrefix(listUnsub, "<https://") || strings.HasPrefix(listUnsub, "<http://"),
        "List-Unsubscribe value must be wrapped URL: %s", listUnsub)

    // 6. Extract token from URL inside <...>.
    raw := strings.TrimPrefix(listUnsub, "<")
    raw = strings.TrimSuffix(raw, ">")
    parts := strings.Split(raw, "/api/v1/u/")
    require.Len(t, parts, 2, "URL must contain /api/v1/u/: %s", raw)
    token := parts[1]
    require.NotEmpty(t, token)

    // 7. POST one-click — must be 200, idempotent.
    resp1 := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token, nil)
    require.Equal(t, 200, resp1.StatusCode)
    resp2 := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token, nil)
    require.Equal(t, 200, resp2.StatusCode, "one-click must be idempotent")

    // 8. Send a second email of the SAME type — must be suppressed (no delivery).
    mailpit.ClearMessages()
    cli.SendEmail(t, SendEmailInput{
        TemplateTypeSlug: typeSlug,
        TemplateSlug:     "unsub-bulk",
        Recipient:        recipient,
        Variables:        map[string]any{"name": "Again"},
    })
    time.Sleep(2 * time.Second)
    msgs := mailpit.GetMessages()
    for _, m := range msgs {
        require.False(t, strings.Contains(strings.Join(m.To, ","), recipient),
            "second send to %s must be suppressed", recipient)
    }

    // 9. Send to a DIFFERENT recipient of the same type — must be delivered.
    other := fmt.Sprintf("other+%d@e2e.test", time.Now().UnixNano())
    cli.SendEmail(t, SendEmailInput{
        TemplateTypeSlug: typeSlug,
        TemplateSlug:     "unsub-bulk",
        Recipient:        other,
        Variables:        map[string]any{"name": "Other"},
    })
    require.Eventually(t, func() bool {
        for _, m := range mailpit.GetMessages() {
            if strings.Contains(strings.Join(m.To, ","), other) {
                return true
            }
        }
        return false
    }, 30*time.Second, 250*time.Millisecond, "uninvolved recipient must still receive")
}

func TestUnsubscribe_TransactionalTemplate_NoHeaders(t *testing.T) {
    EnsureSetup(t)
    cli := NewTestClient(t)
    mailpit := NewMailpitClient(t)
    mailpit.ClearMessages()

    typeSlug := fmt.Sprintf("unsub-tx-%d", time.Now().UnixNano())
    cli.CreateTemplateType(t, TemplateTypeInput{
        Slug: typeSlug, Name: "Transactional", IsBulk: false,
    })
    cli.CreateAndPublishTemplate(t, typeSlug, "tx",
        `<mjml><mj-body><mj-section><mj-column><mj-text>Hi</mj-text></mj-column></mj-section></mj-body></mjml>`)

    recipient := fmt.Sprintf("tx+%d@e2e.test", time.Now().UnixNano())
    cli.SendEmail(t, SendEmailInput{
        TemplateTypeSlug: typeSlug, TemplateSlug: "tx", Recipient: recipient,
    })
    var msgID string
    require.Eventually(t, func() bool {
        for _, m := range mailpit.GetMessages() {
            if strings.Contains(strings.Join(m.To, ","), recipient) {
                msgID = m.ID
                return true
            }
        }
        return false
    }, 30*time.Second, 250*time.Millisecond)

    msg := mailpit.GetMessage(msgID)
    for _, h := range msg.Headers {
        require.False(t,
            strings.EqualFold(h.Name, "List-Unsubscribe") || strings.EqualFold(h.Name, "List-Unsubscribe-Post"),
            "transactional templates must NOT carry List-Unsubscribe headers, found %s", h.Name)
    }
}

func TestUnsubscribe_OptOutAll_BlocksAllTypes(t *testing.T) {
    EnsureSetup(t)
    cli := NewTestClient(t)
    mailpit := NewMailpitClient(t)
    mailpit.ClearMessages()

    // Create two bulk types.
    type1 := fmt.Sprintf("all-1-%d", time.Now().UnixNano())
    type2 := fmt.Sprintf("all-2-%d", time.Now().UnixNano())
    for _, slug := range []string{type1, type2} {
        cli.CreateTemplateType(t, TemplateTypeInput{Slug: slug, Name: slug, IsBulk: true})
        cli.CreateAndPublishTemplate(t, slug, slug,
            `<mjml><mj-body><mj-section><mj-column><mj-text>{{ system.unsubscribe_url }}</mj-text></mj-column></mj-section></mj-body></mjml>`)
    }

    recipient := fmt.Sprintf("all+%d@e2e.test", time.Now().UnixNano())

    // Send first type.
    cli.SendEmail(t, SendEmailInput{TemplateTypeSlug: type1, TemplateSlug: type1, Recipient: recipient})
    var msgID string
    require.Eventually(t, func() bool {
        for _, m := range mailpit.GetMessages() {
            if strings.Contains(strings.Join(m.To, ","), recipient) {
                msgID = m.ID
                return true
            }
        }
        return false
    }, 30*time.Second, 250*time.Millisecond)

    msg := mailpit.GetMessage(msgID)
    var token string
    for _, h := range msg.Headers {
        if strings.EqualFold(h.Name, "List-Unsubscribe") {
            url := strings.TrimSuffix(strings.TrimPrefix(h.Value, "<"), ">")
            token = strings.Split(url, "/api/v1/u/")[1]
        }
    }
    require.NotEmpty(t, token)

    // Opt out of ALL.
    resp := cli.RawHTTP(t, http.MethodPost, "/api/v1/u/"+token+"/all", nil)
    require.Equal(t, 200, resp.StatusCode)

    // Send second (different) type — must be blocked.
    mailpit.ClearMessages()
    cli.SendEmail(t, SendEmailInput{TemplateTypeSlug: type2, TemplateSlug: type2, Recipient: recipient})
    time.Sleep(2 * time.Second)
    for _, m := range mailpit.GetMessages() {
        require.False(t, strings.Contains(strings.Join(m.To, ","), recipient),
            "after opt-out-all, NO type may reach this recipient")
    }
}
```

- [ ] **Step 2: Extend `helpers.go` with the new helpers used above**

Add to `test/e2e/helpers.go`:

```go
type TemplateTypeInput struct {
    Slug   string
    Name   string
    IsBulk bool
}

func (c *TestClient) CreateTemplateType(t *testing.T, in TemplateTypeInput) {
    body := map[string]any{"slug": in.Slug, "name": in.Name, "is_bulk": in.IsBulk}
    resp := c.PostJSON(t, fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/template-types", DefaultTenantCode, DefaultWorkspaceCode), body)
    if resp.StatusCode != 201 && resp.StatusCode != 409 {
        t.Fatalf("CreateTemplateType: status %d", resp.StatusCode)
    }
}

type SendEmailInput struct {
    TemplateTypeSlug string
    TemplateSlug     string
    Recipient        string
    Variables        map[string]any
}

func (c *TestClient) SendEmail(t *testing.T, in SendEmailInput) {
    body := map[string]any{
        "template_type_slug": in.TemplateTypeSlug,
        "template_slug":      in.TemplateSlug,
        "to":                 []string{in.Recipient},
        "variables":          in.Variables,
    }
    resp := c.PostJSON(t, "/api/v1/send", body)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        t.Fatalf("SendEmail: status %d", resp.StatusCode)
    }
}

func (c *TestClient) CreateAndPublishTemplate(t *testing.T, typeSlug, slug, mjml string) {
    // ... POST to /api/v1/manage/.../templates with type_slug, slug, body_mjml
    // ... POST publish endpoint
    // (full implementation: mirror existing CreateTemplate helpers in this file)
}
```

(Adapt to whatever helpers the existing tests use. The body shape for `is_bulk` requires that the manage handler accept the field — if it does not yet, this is a one-line change in `request/template_type.go` and `service/template_type.go.Create`.)

- [ ] **Step 3: Run the test**

```bash
make test-e2e ARGS='-run TestUnsubscribe'
```

Expected: 3 tests PASS. Stack auto-managed by harness.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/unsubscribe_e2e_test.go test/e2e/helpers.go
git commit -m "test(e2e): unsubscribe headers + one-click + opt-out-all flows"
```

---

## Task 19: Local UI E2E test (build tag `e2e_local`)

**Files:**
- Create: `test/e2e/unsubscribe_ui_test.go`

This test asks the engineer to have `make dev` running locally (full stack including Next.js frontend), then drives the actual Next.js pages with chromedp. It is the literal user requirement: "levanta local, configura mailpit (usa docker), prueba e2e completa con validación UI e interactuando con el unsubscribe."

The build tag `e2e_local` keeps it out of CI (`make test-e2e` uses tag `e2e`).

- [ ] **Step 1: Document and write the test**

Path: `test/e2e/unsubscribe_ui_test.go`

```go
//go:build e2e_local

// To run this test:
//   1. In one terminal:  make dev    # starts docker stack (postgres, mailpit, app, keycloak) and `pnpm --dir web dev`
//   2. Wait for Next.js to print "Ready on http://localhost:3000"
//   3. In another terminal:
//        export SENDA_E2E_LOCAL_API_KEY=<api key from /api/v1/manage/.../api-keys>
//        export SENDA_E2E_LOCAL_BACKEND=http://localhost:8081
//        export SENDA_E2E_LOCAL_FRONTEND=http://localhost:3000
//        export SENDA_E2E_LOCAL_MAILPIT=http://localhost:8026
//        go test -tags=e2e_local -v -timeout 300s ./test/e2e -run TestUnsubscribeUI
//
// The test will:
//   - create a tenant/workspace if missing (idempotent)
//   - create a bulk template_type and template
//   - send an email to a unique address
//   - poll Mailpit for the message
//   - extract the unsubscribe token
//   - drive Next.js /u/{token} with chromedp: assert title, click "this event", confirm
//   - drive /u/{token}/preferences: toggle a checkbox, save
//   - send another email and assert it is blocked
//   - opt-in via preferences, send again, assert delivery resumes

package e2e

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/chromedp/chromedp"
    "github.com/stretchr/testify/require"
)

func envOr(k, def string) string {
    if v := os.Getenv(k); v != "" { return v }
    return def
}

func TestUnsubscribeUI_FullFlow(t *testing.T) {
    backend  := envOr("SENDA_E2E_LOCAL_BACKEND",  "http://localhost:8081")
    frontend := envOr("SENDA_E2E_LOCAL_FRONTEND", "http://localhost:3000")
    mailpit  := envOr("SENDA_E2E_LOCAL_MAILPIT",  "http://localhost:8026")
    apiKey   := os.Getenv("SENDA_E2E_LOCAL_API_KEY")
    require.NotEmpty(t, apiKey, "set SENDA_E2E_LOCAL_API_KEY (workspace api key)")

    // 0. Sanity: backend, mailpit and frontend up?
    require.NoError(t, ping(backend+"/healthz"), "backend not reachable")
    require.NoError(t, ping(mailpit+"/api/v1/messages"), "mailpit not reachable")
    require.NoError(t, ping(frontend+"/"), "frontend (pnpm --dir web dev) not reachable")

    // 1. Clear mailpit.
    clearMailpit(t, mailpit)

    // 2. Create bulk template_type + template via API key.
    typeSlug := fmt.Sprintf("ui-bulk-%d", time.Now().UnixNano())
    createBulkTemplateType(t, backend, apiKey, typeSlug)
    createTemplate(t, backend, apiKey, typeSlug, "ui-bulk", `<mjml><mj-body><mj-section><mj-column>
        <mj-text>Hello {{ event.name }}</mj-text>
        <mj-text>Unsub: {{ system.unsubscribe_url }}</mj-text>
    </mj-column></mj-section></mj-body></mjml>`)

    // 3. Send.
    recipient := fmt.Sprintf("ui-recipient+%d@e2e.test", time.Now().UnixNano())
    sendEmail(t, backend, apiKey, typeSlug, "ui-bulk", recipient)

    // 4. Wait for the message and extract token.
    token := waitForUnsubscribeToken(t, mailpit, recipient, 30*time.Second)

    // 5. Drive UI: navigate to /u/{token}.
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()
    ctx, cancel2 := context.WithTimeout(ctx, 60*time.Second)
    defer cancel2()

    var titleText string
    require.NoError(t, chromedp.Run(ctx,
        chromedp.Navigate(frontend+"/u/"+token),
        chromedp.WaitVisible(`[data-testid="unsubscribe-card"]`, chromedp.ByQuery),
        chromedp.Text(`[data-testid="unsubscribe-card"] h2, [data-testid="unsubscribe-card"] [class*="card-title"]`, &titleText, chromedp.ByQuery),
        chromedp.Click(`[data-testid="radio-this-event"]`, chromedp.ByQuery),
        chromedp.Click(`[data-testid="confirm-button"]`, chromedp.ByQuery),
        chromedp.WaitVisible(`[data-testid="success-card"]`, chromedp.ByQuery),
    ))
    require.Contains(t, strings.ToLower(titleText), "unsubscrib", "card title must mention unsubscribe; got: %s", titleText)

    // 6. Send a second email of the same type — must be blocked.
    clearMailpit(t, mailpit)
    sendEmail(t, backend, apiKey, typeSlug, "ui-bulk", recipient)
    time.Sleep(3 * time.Second)
    msgs := getMailpitMessages(t, mailpit)
    require.Empty(t, filterByRecipient(msgs, recipient), "after opt-out, no email of this type may reach the recipient")

    // 7. Open preference center, re-subscribe, save.
    require.NoError(t, chromedp.Run(ctx,
        chromedp.Navigate(frontend+"/u/"+token+"/preferences"),
        chromedp.WaitVisible(`[data-testid="preferences-card"]`, chromedp.ByQuery),
        chromedp.WaitVisible(fmt.Sprintf(`[data-testid="pref-cb-%s"]`, typeSlug), chromedp.ByQuery),
        chromedp.Click(fmt.Sprintf(`[data-testid="pref-cb-%s"]`, typeSlug), chromedp.ByQuery),
        chromedp.Click(`[data-testid="save-button"]`, chromedp.ByQuery),
        chromedp.Sleep(500*time.Millisecond),
    ))

    // 8. Send again — must be delivered now.
    clearMailpit(t, mailpit)
    sendEmail(t, backend, apiKey, typeSlug, "ui-bulk", recipient)
    require.Eventually(t, func() bool {
        return len(filterByRecipient(getMailpitMessages(t, mailpit), recipient)) > 0
    }, 30*time.Second, 250*time.Millisecond, "after re-subscribe, email must arrive")
}

// --- helpers ---

func ping(url string) error {
    resp, err := http.Get(url)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 500 { return fmt.Errorf("status %d", resp.StatusCode) }
    return nil
}

type mailpitMsg struct {
    ID  string   `json:"ID"`
    To  []struct{ Address string `json:"Address"` } `json:"To"`
}
type mailpitListing struct {
    Messages []mailpitMsg `json:"messages"`
}

func clearMailpit(t *testing.T, base string) {
    req, _ := http.NewRequest(http.MethodDelete, base+"/api/v1/messages", nil)
    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    resp.Body.Close()
}

func getMailpitMessages(t *testing.T, base string) []mailpitMsg {
    resp, err := http.Get(base + "/api/v1/messages")
    require.NoError(t, err)
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var l mailpitListing
    require.NoError(t, jsonUnmarshal(body, &l))
    return l.Messages
}

func filterByRecipient(msgs []mailpitMsg, recipient string) []mailpitMsg {
    var out []mailpitMsg
    for _, m := range msgs {
        for _, t := range m.To {
            if t.Address == recipient { out = append(out, m); break }
        }
    }
    return out
}

func waitForUnsubscribeToken(t *testing.T, mailpit, recipient string, timeout time.Duration) string {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        msgs := filterByRecipient(getMailpitMessages(t, mailpit), recipient)
        if len(msgs) > 0 {
            // fetch full message
            resp, err := http.Get(mailpit + "/api/v1/message/" + msgs[0].ID)
            require.NoError(t, err)
            body, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            var full struct {
                Headers map[string][]string `json:"Headers"`
            }
            require.NoError(t, jsonUnmarshal(body, &full))
            for k, vs := range full.Headers {
                if !strings.EqualFold(k, "List-Unsubscribe") { continue }
                for _, v := range vs {
                    raw := strings.TrimSuffix(strings.TrimPrefix(v, "<"), ">")
                    if i := strings.Index(raw, "/api/v1/u/"); i >= 0 {
                        return raw[i+len("/api/v1/u/"):]
                    }
                }
            }
        }
        time.Sleep(250 * time.Millisecond)
    }
    t.Fatalf("token never appeared in headers")
    return ""
}

// API helpers (createBulkTemplateType, createTemplate, sendEmail, jsonUnmarshal):
//   ... small wrappers around backend HTTP using the api key bearer header.
//   ... idempotent: 201 or 409 OK.
//   ... full code mirrors existing helpers; keep it local to this file to avoid
//   ... cross-tag imports.
```

(The helpers `createBulkTemplateType`, `createTemplate`, `sendEmail`, `jsonUnmarshal` are short JSON-over-HTTP wrappers — implement them in the same file. They use stdlib `encoding/json` and `net/http` only.)

- [ ] **Step 2: Add a Make target**

Edit `Makefile`. Add:

```makefile
test-ui-unsubscribe: ## Run the local UI E2E for unsubscribe (requires `make dev` running and SENDA_E2E_LOCAL_API_KEY set)
	go test -tags=e2e_local -v -count=1 -timeout 300s ./test/e2e -run TestUnsubscribeUI
```

- [ ] **Step 3: Manual verification — first run**

In terminal 1:
```bash
make dev
```

Wait for Next.js to print `Ready on http://localhost:3000` and Senda to print `listening on :8081`.

In terminal 2:
```bash
# Get an api key (one-time setup; store somewhere)
export SENDA_E2E_LOCAL_API_KEY=$(./scripts/dev/get-or-create-api-key.sh)  # or follow docs/DEVELOPMENT.md
make test-ui-unsubscribe
```

Expected: PASS. The test will open a headless Chrome window (chromedp default), navigate to `/u/{token}`, click radio, click confirm, then visit `/u/{token}/preferences`, toggle the checkbox, save. Final assertion: after re-subscribe, the new email is delivered.

If chromedp cannot find Chrome on PATH, set `CHROMEDP_NO_SANDBOX=1` and ensure `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` (mac) or `/usr/bin/google-chrome` (linux) exists.

- [ ] **Step 4: Tear down**

```bash
make dev-down
```

- [ ] **Step 5: Commit**

```bash
git add test/e2e/unsubscribe_ui_test.go Makefile
git commit -m "test(e2e_local): full UI flow for unsubscribe with chromedp + mailpit"
```

---

## Task 20: Documentation and skill references

**Files:**
- Modify: `docs/EMAIL_FLOWS.md`
- Modify: `docs/specs/SECURITY_CHECKLIST.md`
- Modify: `skills/senda/references/sending-emails.md`
- Modify: `skills/senda/references/building-a-template.md`
- Modify: `skills/senda/SKILL.md`

- [ ] **Step 1: Add unsubscribe section to `docs/EMAIL_FLOWS.md`**

Append a new section at the end:

```markdown
## Unsubscribe

When a `template_type` has `is_bulk = true`, every send for that type carries:

- HTTP header `List-Unsubscribe: <https://<base>/api/v1/u/{token}>` (RFC 2369)
- HTTP header `List-Unsubscribe-Post: List-Unsubscribe=One-Click` (RFC 8058)
- Two template variables resolvable in MJML: `{{ system.unsubscribe_url }}` and `{{ system.preferences_url }}`

The token is HMAC-SHA256 over a JSON payload (`v`, `ws`, `tt`, `ttn`, `e`, `eid`, `iat`, `exp`) signed
with the per-workspace key stored in `workspaces.unsubscribe_signing_key`. Tokens expire 12 months
after issue.

Suppression has three levels checked in cascade by `SuppressionBatchEvaluator.EvaluateForType`:

1. **`suppression_global`** — hard bounce / complaint anywhere.
2. **`suppression_workspace`** — opt-out-all (`reason='unsubscribe'`) or admin block.
3. **`template_type_subscription`** — per-type opt-out from preference center or one-click.

The first match blocks the send. `template_type` transactional sends (where `is_bulk = false`) are
**never** blocked by level 3 and **never** carry the `List-Unsubscribe` header.

Recipients can self-service:

- `POST /api/v1/u/{token}` — one-click opt-out from this type (RFC 8058).
- `POST /api/v1/u/{token}/all` — opt-out from all types in this workspace.
- `POST /api/v1/u/{token}/resubscribe` — undo opt-out-all (does NOT undo hard_bounce/complaint).
- `GET  /api/v1/u/{token}/preferences` — list types received in last 12 months + state.
- `POST /api/v1/u/{token}/preferences` — flip subscription state per type.

The browser-facing pages live in Next.js: `/u/{token}` (single-event vs all radio), `/u/{token}/preferences`
(checkbox-per-type center).
```

- [ ] **Step 2: Add to `docs/specs/SECURITY_CHECKLIST.md`**

Insert a new row in the appropriate table:

```markdown
| Unsubscribe token | HMAC-SHA256, per-workspace 32-byte signing key, 12-month TTL, idempotent verify | Constant-time compare via `hmac.Equal`. No DB writes on token verification failure. |
```

- [ ] **Step 3: Update `skills/senda/references/sending-emails.md`**

Add to the "Template types" section:

```markdown
### Bulk vs transactional

Set `is_bulk: true` when creating a `template_type` for newsletters, marketing, alerts, or any
recurring communication that the recipient should be able to opt out of. Senda will:

- inject `List-Unsubscribe` and `List-Unsubscribe-Post` headers on every send;
- expose `{{ system.unsubscribe_url }}` and `{{ system.preferences_url }}` template variables;
- consult `template_type_subscription` before sending.

Leave `is_bulk` at the default `false` for transactional emails (password reset, OTP, receipts,
critical account events). These never carry an unsubscribe header and never check
`template_type_subscription`.
```

- [ ] **Step 4: Update `skills/senda/references/building-a-template.md`**

Add to the variables table:

```markdown
| `{{ system.unsubscribe_url }}` | URL to the unsubscribe landing page. Empty when `is_bulk = false`. Renders to a one-time signed link bound to the recipient. |
| `{{ system.preferences_url }}` | URL to the preference center. Empty when `is_bulk = false`. Same token base; recipient can manage all subscriptions. |
| `{{ system.workspace_name }}`  | Human-readable workspace name. Useful for footer branding. Empty when `is_bulk = false`. |
```

- [ ] **Step 5: Update `skills/senda/SKILL.md` topology if needed**

If the SKILL.md mentions the public route surface, add `/api/v1/u/...` to the list with the note "public, unauthenticated, used for RFC 8058 unsubscribe flow."

- [ ] **Step 6: Commit**

```bash
git add docs/EMAIL_FLOWS.md docs/specs/SECURITY_CHECKLIST.md \
        skills/senda/references/sending-emails.md skills/senda/references/building-a-template.md \
        skills/senda/SKILL.md
git commit -m "docs(unsubscribe): document headers, endpoints, system vars and is_bulk semantics"
```

---

## Task 21: Final gate

**Files:**
- (no files in this task)

- [ ] **Step 1: Run minimum local gate**

```bash
make lint
make vet
make test
```

Expected: all PASS.

- [ ] **Step 2: Run frontend gate**

```bash
pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web test
```

Expected: all PASS.

- [ ] **Step 3: Run backend PR gate**

```bash
make ci-backend-pr
```

Expected: PASS.

- [ ] **Step 4: Run full PR gate**

```bash
make ci-pr
```

Expected: PASS.

- [ ] **Step 5: Run taxonomy check**

```bash
make ci-taxonomy-check
```

Expected: PASS.

- [ ] **Step 6: Run integration tests for new stores**

```bash
make test-integration
```

Expected: PASS, including `TestTTSStore_*` and `TestEmailStore_DistinctTemplateTypesForRecipient`.

- [ ] **Step 7: Run e2e backend tests**

```bash
make test-e2e ARGS='-run TestUnsubscribe'
```

Expected: PASS, all 3 unsubscribe e2e tests.

- [ ] **Step 8: Run UI e2e (requires manual setup)**

Open one terminal:
```bash
make dev
```

Open second terminal (after Next.js shows ready):
```bash
export SENDA_E2E_LOCAL_API_KEY=<your dev workspace api key>
make test-ui-unsubscribe
```

Expected: PASS. chromedp drives the live frontend, mailpit captures emails.

- [ ] **Step 9: Open PR**

```bash
git push -u origin feat/email-unsubscribe
gh pr create --base main --title "feat: email unsubscribe (RFC 8058 + preference center)" --body "$(cat <<'EOF'
## Summary

- Adds RFC 8058 / Gmail-Yahoo-bulk-sender compliant unsubscribe with three suppression levels (global, workspace, per-template_type), reversible preference center, and HMAC-signed per-workspace tokens.
- Public backend endpoints under `/api/v1/u/...`, public Next.js pages under `/u/[token]` and `/u/[token]/preferences`.
- Headers `List-Unsubscribe` and `List-Unsubscribe-Post` injected automatically when `template_type.is_bulk = true`.
- Variables `{{ system.unsubscribe_url }}` and `{{ system.preferences_url }}` exposed to template authors.

## Test plan

- [x] `make lint && make vet && make test` (unit + race)
- [x] `pnpm --dir web typecheck && pnpm --dir web lint && pnpm --dir web test`
- [x] `make test-integration` covers Postgres stores
- [x] `make test-e2e ARGS='-run TestUnsubscribe'` covers headers + one-click + opt-out-all + transactional negative
- [x] `make test-ui-unsubscribe` (manual, with `make dev`): full UI flow with chromedp clicks
- [x] `make ci-pr` and `make ci-taxonomy-check`
EOF
)"
```

- [ ] **Step 10: Wait for CI and merge**

```bash
gh pr checks --watch
gh pr merge --squash --delete-branch
git checkout main && git pull --ff-only
```

---

## Self-Review Checklist (engineer-runs-mentally before starting)

- Migration `000048` is idempotent against existing workspaces (UPDATE populates `unsubscribe_signing_key` for pre-existing rows).
- `ALTER TYPE ... ADD VALUE IF NOT EXISTS` works in Postgres 16 inside a transaction without a separate commit because the value is not referenced in DML in the same migration.
- `SuppressionBatchEvaluator.EvaluateForType` is backward compatible: callers that do not pass a `WithTemplateTypeStore` get the old behaviour (no level 3 check).
- Token is workspace-scoped: a leaked dump of one workspace's signing key only compromises that workspace's links.
- Constant-time HMAC compare via `hmac.Equal` blocks timing oracles.
- One-click POST is idempotent (UPSERT in `template_type_subscription`).
- Transactional templates (`is_bulk = false`) never carry `List-Unsubscribe` (compliance — these are not bulk and adding the header is misleading).
- Hard bounce / complaint cannot be undone by the recipient: `Resubscribe` only removes `reason='unsubscribe'` rows; `RemovalReason='recipient_resubscribe'`. Admin can override via existing manage endpoints with audit.
- Preference center only lists types received in the last 12 months — no leak of the workspace's full template_type catalog.
- `data-testid` attributes on all clickable UI elements so chromedp E2E does not depend on text or class names.
- E2E tests cover: header presence, header absence (transactional), one-click idempotency, opt-out-all blocks all types, opt-in restores delivery, UI click flow.
