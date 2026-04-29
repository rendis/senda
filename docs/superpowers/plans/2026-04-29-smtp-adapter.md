# SMTP Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SMTP as a first-class Senda adapter that supports plain local/internal relays and authenticated SMTP with STARTTLS or implicit TLS, then expose it in the existing Adapters frontend module.

**Architecture:** Backend owns the canonical SMTP contract, DB enum, encrypted config, sender runtime, and default sender identity behavior. Frontend extends the existing Adapters module to create/edit/test SMTP adapters using that contract, without adding a separate SMTP page. SMTP does not implement provider identity sync; it seeds and maintains a manual default email identity from `from_email`.

**Tech Stack:** Go 1.25, PostgreSQL migrations, Echo handlers, River send worker, `net/smtp`, `crypto/tls`, Next.js 16, React 19, TypeScript, Tailwind, TanStack Query.

---

## Contract

Create/update adapter request:

```json
{
  "name": "SMTP Production",
  "adapter_type": "smtp",
  "config": {
    "host": "smtp.example.com",
    "port": 587,
    "tls_mode": "starttls",
    "auth_mode": "plain",
    "username": "user-or-apikey",
    "password": "secret",
    "from_email": "no-reply@example.com",
    "from_name": "Senda"
  },
  "is_default": false,
  "rate_limit_per_second": 10
}
```

Rules:

- `adapter_type` accepts `smtp`.
- `config.host`, `config.port`, `config.tls_mode`, and `config.from_email` are required for SMTP.
- `config.tls_mode` accepts `none`, `starttls`, or `implicit_tls`.
- `config.auth_mode` is optional and accepts `plain` or `login`; default is `plain` when credentials are present.
- `config.username` and `config.password` are optional. If either is set on create, both must be set.
- On update, omitting or passing an empty `password` keeps the existing password. Passing a non-empty `password` replaces it.
- `config.from_name` is optional.
- `config_meta` may expose only `host`, `port`, `tls_mode`, and `from_email`.
- SMTP config, including `password`, stays encrypted in `adapters.config_encrypted`.
- SMTP creates or updates a manual default email identity from `from_email`; provider sync remains unsupported.

---

## File Structure

Backend files:

- Modify `migrations/000002_enums.up.sql` and `migrations/000002_enums.down.sql` only if the project accepts editing early migrations in local dev. Otherwise create a new migration such as `migrations/000042_add_smtp_adapter_type.up.sql` and matching down migration. Preferred: create migration `000042` to avoid rewriting applied history.
- Modify `internal/domain/adapter.go` to add `AdapterTypeSMTP`.
- Modify `internal/adapter/smtp/adapter.go` to add SMTP config, validation, auth, STARTTLS, and implicit TLS.
- Modify `internal/adapter/smtp/adapter_test.go` to cover config validation and sender construction behavior.
- Modify `internal/adapter/river/send_worker.go` to add SMTP in `DefaultAdapterSenderFactory`.
- Modify `internal/adapter/river/send_worker_test.go` to assert SMTP factory support.
- Modify `internal/http/handler/adapter.go` to validate SMTP config, expose safe config meta, preserve password on update, and seed default SMTP identity.
- Modify `internal/http/handler/adapter_test.go` to cover SMTP create/update/test-send validation.
- Modify `internal/resolution/from_resolver.go` and `internal/service/from_email.go` so SMTP can fall back to `from_email` when no DB default identity exists.
- Modify `internal/service/identity.go` to allow SMTP manual identities without provider-domain validation.
- Modify `internal/app/identity_factory.go` tests only if they assert the unsupported type string directly.
- Modify docs/API/Postman/OpenAPI after backend contract is final.

Frontend files:

- Modify `web/src/types/adapters.ts` to add `smtp` and `SmtpConfig`.
- Modify `web/src/components/adapters/adapter-form.tsx` to render SMTP fields and build the SMTP request.
- Modify `web/src/components/adapters/adapter-type-badge.tsx` to add SMTP badge.
- Modify `web/src/components/adapters/adapters-content.tsx` to hide SES/Gmail-only actions for SMTP and support SMTP test send.
- Modify `web/src/hooks/use-adapters.ts` only if request/response types need frontend-specific helpers.
- Add focused tests beside existing frontend tests or create helper tests if the component is not currently easy to render.
- DoD E2E data for final UI flow: host `127.0.0.1`, port `2525`, auth `PLAIN` or `LOGIN`, username from the user's provided SMTP relay, password from the user's provided SMTP relay, TLS mode `none`, from `noreply-senda@tether.education`, recipient `reynaldo@tether.education`. Do not print the password in logs, commits, docs, or final summaries.

---

## Issue 1: Backend/API/Modelo

### Task 1: Add SMTP Adapter Type to Domain and Database

**Files:**
- Create: `migrations/000042_add_smtp_adapter_type.up.sql`
- Create: `migrations/000042_add_smtp_adapter_type.down.sql`
- Modify: `internal/domain/adapter.go`
- Test: `internal/adapter/postgres/adapter_repo_test.go`

- [ ] **Step 1: Write the failing repository test**

Add this test to `internal/adapter/postgres/adapter_repo_test.go`:

```go
func TestAdapterRepo_CreateAndGet_SMTP(t *testing.T) {
	ctx := context.Background()
	deps := setupAdapterRepoTest(t)

	adapter := &domain.Adapter{
		ID:                 uuid.Must(uuid.NewV7()),
		WorkspaceID:        &deps.workspaceID,
		Name:               "SMTP Relay",
		AdapterType:        domain.AdapterTypeSMTP,
		ConfigEncrypted:    []byte(`{"host":"localhost","port":1025,"tls_mode":"none","from_email":"no-reply@example.com"}`),
		IsDefault:          false,
		RateLimitPerSecond: 10,
		ConfigMeta: map[string]string{
			"host":       "localhost",
			"port":       "1025",
			"tls_mode":   "none",
			"from_email": "no-reply@example.com",
		},
	}

	if err := deps.repo.Create(ctx, adapter); err != nil {
		t.Fatalf("Create() SMTP error = %v", err)
	}

	got, err := deps.repo.GetByID(ctx, adapter.ID)
	if err != nil {
		t.Fatalf("GetByID() SMTP error = %v", err)
	}
	if got.AdapterType != domain.AdapterTypeSMTP {
		t.Fatalf("AdapterType = %q, want %q", got.AdapterType, domain.AdapterTypeSMTP)
	}
	if got.ConfigMeta["tls_mode"] != "none" {
		t.Fatalf("ConfigMeta[tls_mode] = %q, want none", got.ConfigMeta["tls_mode"])
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/adapter/postgres -run TestAdapterRepo_CreateAndGet_SMTP -count=1
```

Expected: FAIL because `domain.AdapterTypeSMTP` is undefined or the DB enum rejects `smtp`.

- [ ] **Step 3: Add the domain constant**

Change `internal/domain/adapter.go`:

```go
const (
	AdapterTypeSES   AdapterType = "ses"
	AdapterTypeGmail AdapterType = "gmail"
	AdapterTypeSMTP  AdapterType = "smtp"
)
```

- [ ] **Step 4: Add the migration**

Create `migrations/000042_add_smtp_adapter_type.up.sql`:

```sql
ALTER TYPE adapter_type ADD VALUE IF NOT EXISTS 'smtp';
```

Create `migrations/000042_add_smtp_adapter_type.down.sql`:

```sql
-- PostgreSQL cannot remove enum values safely without recreating dependent columns.
-- Keep this migration irreversible; rolling back code should simply stop creating smtp adapters.
```

- [ ] **Step 5: Run the repository test**

Run:

```bash
go test ./internal/adapter/postgres -run TestAdapterRepo_CreateAndGet_SMTP -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/000042_add_smtp_adapter_type.up.sql migrations/000042_add_smtp_adapter_type.down.sql internal/domain/adapter.go internal/adapter/postgres/adapter_repo_test.go
git commit -m "feat: add smtp adapter type"
```

### Task 2: Implement SMTP Config and Transport Modes

**Files:**
- Modify: `internal/adapter/smtp/adapter.go`
- Modify: `internal/adapter/smtp/adapter_test.go`

- [ ] **Step 1: Add failing config validation tests**

Add these tests to `internal/adapter/smtp/adapter_test.go`:

```go
func TestConfigValidate_PlainSMTP(t *testing.T) {
	cfg := Config{
		Host:      "localhost",
		Port:      1025,
		TLSMode:   TLSModeNone,
		FromEmail: "no-reply@example.com",
	}
	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_AuthenticatedSMTPRequiresUsernameAndPasswordTogether(t *testing.T) {
	cfg := Config{
		Host:      "smtp.example.com",
		Port:      587,
		TLSMode:   TLSModeStartTLS,
		Username:  "apikey",
		FromEmail: "no-reply@example.com",
	}
	require.ErrorContains(t, cfg.Validate(), "smtp username and password must be provided together")
}

func TestConfigValidate_RejectsUnknownTLSMode(t *testing.T) {
	cfg := Config{
		Host:      "smtp.example.com",
		Port:      587,
		TLSMode:   TLSMode("ssl-ish"),
		FromEmail: "no-reply@example.com",
	}
	require.ErrorContains(t, cfg.Validate(), "invalid SMTP tls_mode")
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
go test ./internal/adapter/smtp -run 'TestConfigValidate' -count=1
```

Expected: FAIL because `Config`, `TLSModeNone`, and related symbols are undefined.

- [ ] **Step 3: Add Config, TLSMode, and constructor**

Update `internal/adapter/smtp/adapter.go` with these definitions:

```go
type TLSMode string

const (
	TLSModeNone        TLSMode = "none"
	TLSModeStartTLS    TLSMode = "starttls"
	TLSModeImplicitTLS TLSMode = "implicit_tls"
)

type Config struct {
	Host      string  `json:"host"`
	Port      int     `json:"port"`
	TLSMode   TLSMode `json:"tls_mode"`
	AuthMode  string  `json:"auth_mode,omitempty"`
	Username  string  `json:"username,omitempty"`
	Password  string  `json:"password,omitempty"`
	FromEmail string  `json:"from_email"`
	FromName  string  `json:"from_name,omitempty"`
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("missing SMTP host")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid SMTP port")
	}
	switch c.TLSMode {
	case TLSModeNone, TLSModeStartTLS, TLSModeImplicitTLS:
	case "":
		return fmt.Errorf("missing SMTP tls_mode")
	default:
		return fmt.Errorf("invalid SMTP tls_mode %q", c.TLSMode)
	}
	if strings.TrimSpace(c.FromEmail) == "" {
		return fmt.Errorf("missing SMTP from_email")
	}
	if c.AuthMode != "" && c.AuthMode != "plain" && c.AuthMode != "login" {
		return fmt.Errorf("invalid SMTP auth_mode %q", c.AuthMode)
	}
	if (c.Username == "") != (c.Password == "") {
		return fmt.Errorf("smtp username and password must be provided together")
	}
	return nil
}

func NewAdapterFromConfig(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Adapter{cfg: cfg}, nil
}
```

Change the adapter struct and legacy constructor:

```go
type Adapter struct {
	cfg Config
}

func NewAdapter(host string, port int) *Adapter {
	return &Adapter{cfg: Config{Host: host, Port: port, TLSMode: TLSModeNone}}
}
```

Add imports:

```go
import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)
```

- [ ] **Step 4: Implement send paths**

Replace the body of `Send` and add helpers in `internal/adapter/smtp/adapter.go`:

```go
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("smtp: build message: %w", err)
	}

	addr := net.JoinHostPort(a.cfg.Host, strconv.Itoa(a.cfg.Port))
	recipients := allRecipients(msg)
	auth := a.auth()

	switch a.cfg.TLSMode {
	case TLSModeImplicitTLS:
		err = a.sendImplicitTLS(ctx, addr, auth, msg.From.Address, recipients, rawMsg)
	default:
		err = smtp.SendMail(addr, auth, msg.From.Address, recipients, rawMsg)
	}
	if err != nil {
		return "", fmt.Errorf("smtp: send: %w", err)
	}

	return fmt.Sprintf("<trk-%s@senda>", msg.TrackingID), nil
}

func (a *Adapter) auth() smtp.Auth {
	if a.cfg.Username == "" && a.cfg.Password == "" {
		return nil
	}
	if a.cfg.AuthMode == "login" {
		return loginAuth{username: a.cfg.Username, password: a.cfg.Password}
	}
	return smtp.PlainAuth("", a.cfg.Username, a.cfg.Password, a.cfg.Host)
}

type loginAuth struct {
	username string
	password string
}

func (a loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(string(fromServer))
	if strings.Contains(prompt, "password") {
		return []byte(a.password), nil
	}
	return []byte(a.username), nil
}

func (a *Adapter) sendImplicitTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, rawMsg []byte) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: a.cfg.Host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, a.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	return sendWithClient(client, auth, from, to, rawMsg)
}

func sendWithClient(client *smtp.Client, auth smtp.Auth, from string, to []string, rawMsg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(rawMsg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
```

Update `HealthCheck` to use `a.cfg.Host` and `a.cfg.Port`.

- [ ] **Step 5: Run SMTP package tests**

Run:

```bash
go test ./internal/adapter/smtp -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/smtp/adapter.go internal/adapter/smtp/adapter_test.go
git commit -m "feat: support smtp auth and tls modes"
```

### Task 3: Add SMTP to Sender Factory

**Files:**
- Modify: `internal/adapter/river/send_worker.go`
- Modify: `internal/adapter/river/send_worker_test.go`

- [ ] **Step 1: Add failing factory test**

Add this test to `internal/adapter/river/send_worker_test.go`:

```go
func TestDefaultAdapterSenderFactory_SMTP(t *testing.T) {
	adapter := &domain.Adapter{AdapterType: domain.AdapterTypeSMTP}
	cfg := []byte(`{"host":"localhost","port":1025,"tls_mode":"none","from_email":"no-reply@example.com"}`)

	sender, err := DefaultAdapterSenderFactory(context.Background(), adapter, cfg)
	if err != nil {
		t.Fatalf("DefaultAdapterSenderFactory() SMTP error = %v", err)
	}
	if sender.Name() != "smtp" {
		t.Fatalf("sender.Name() = %q, want smtp", sender.Name())
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/adapter/river -run TestDefaultAdapterSenderFactory_SMTP -count=1
```

Expected: FAIL with unsupported adapter type.

- [ ] **Step 3: Add SMTP factory branch**

Update imports in `internal/adapter/river/send_worker.go`:

```go
smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
```

Add this case:

```go
case domain.AdapterTypeSMTP:
	var cfg smtpadapter.Config
	if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
		return nil, fmt.Errorf("%w: unmarshal SMTP config: %w", domain.ErrValidation, err)
	}
	return smtpadapter.NewAdapterFromConfig(cfg)
```

- [ ] **Step 4: Run factory test**

Run:

```bash
go test ./internal/adapter/river -run TestDefaultAdapterSenderFactory_SMTP -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/river/send_worker.go internal/adapter/river/send_worker_test.go
git commit -m "feat: build smtp senders from adapter config"
```

### Task 4: Validate SMTP Config in Adapter Handler

**Files:**
- Modify: `internal/http/handler/adapter.go`
- Modify: `internal/http/handler/adapter_test.go`

- [ ] **Step 1: Add failing create validation test**

Add this test to `internal/http/handler/adapter_test.go` near the create validation tests:

```go
func TestAdapterHandler_Create_SMTPValidatesAndStoresSafeMeta(t *testing.T) {
	h, deps := setupAdapterHandlerTest(t)

	body := `{
		"name":"SMTP Relay",
		"adapter_type":"smtp",
		"config":{
			"host":"smtp.example.com",
			"port":587,
			"tls_mode":"starttls",
			"auth_mode":"plain",
			"username":"apikey",
			"password":"secret",
			"from_email":"no-reply@example.com",
			"from_name":"Senda"
		},
		"rate_limit_per_second":10
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/adapters", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := deps.echo.NewContext(req, rec)
	deps.withWorkspace(c)

	if err := h.Create(&c); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if deps.adapterStore.created.AdapterType != domain.AdapterTypeSMTP {
		t.Fatalf("adapter type = %q, want smtp", deps.adapterStore.created.AdapterType)
	}
	if deps.adapterStore.created.ConfigMeta["host"] != "smtp.example.com" {
		t.Fatalf("host meta = %q", deps.adapterStore.created.ConfigMeta["host"])
	}
	if _, leaked := deps.adapterStore.created.ConfigMeta["password"]; leaked {
		t.Fatal("password must not be stored in config_meta")
	}
}
```

- [ ] **Step 2: Run the failing handler test**

Run:

```bash
go test ./internal/http/handler -run TestAdapterHandler_Create_SMTPValidatesAndStoresSafeMeta -count=1
```

Expected: FAIL because `smtp` is invalid.

- [ ] **Step 3: Import SMTP config and update validation**

Add import:

```go
smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
```

Update `isValidAdapterType`:

```go
case domain.AdapterTypeSES, domain.AdapterTypeGmail, domain.AdapterTypeSMTP:
	return true
```

Update `validateConfig`:

```go
case domain.AdapterTypeSMTP:
	var cfg smtpadapter.Config
	if err := json.Unmarshal(config, &cfg); err != nil {
		errs = append(errs, response.FieldError{Field: "config", Message: "must be a valid SMTP config object"})
		break
	}
	if err := cfg.Validate(); err != nil {
		errs = append(errs, response.FieldError{Field: "config", Message: err.Error()})
	}
```

Update `extractPublicConfigFields`:

```go
case domain.AdapterTypeSMTP:
	if v, ok := cfgMap["host"].(string); ok && v != "" {
		meta["host"] = v
	}
	if v, ok := cfgMap["tls_mode"].(string); ok && v != "" {
		meta["tls_mode"] = v
	}
	if v, ok := cfgMap["from_email"].(string); ok && v != "" {
		meta["from_email"] = v
	}
	switch v := cfgMap["port"].(type) {
	case float64:
		meta["port"] = strconv.Itoa(int(v))
	case string:
		if v != "" {
			meta["port"] = v
		}
	}
```

Add `strconv` import if not already present.

- [ ] **Step 4: Run handler test**

Run:

```bash
go test ./internal/http/handler -run TestAdapterHandler_Create_SMTPValidatesAndStoresSafeMeta -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/http/handler/adapter.go internal/http/handler/adapter_test.go
git commit -m "feat: validate smtp adapter config"
```

### Task 5: Preserve SMTP Password on Update

**Files:**
- Modify: `internal/http/handler/adapter.go`
- Modify: `internal/http/handler/adapter_test.go`

- [ ] **Step 1: Add failing update test**

Add this test to `internal/http/handler/adapter_test.go`:

```go
func TestAdapterHandler_Update_SMTPKeepsPasswordWhenBlank(t *testing.T) {
	h, deps := setupAdapterHandlerTest(t)
	adapterID := uuid.Must(uuid.NewV7())
	deps.crypto.decryptFn = func(_ []byte) ([]byte, error) {
		return []byte(`{"host":"smtp.example.com","port":587,"tls_mode":"starttls","auth_mode":"plain","username":"apikey","password":"old-secret","from_email":"old@example.com"}`), nil
	}
	deps.adapterStore.adapter = &domain.Adapter{
		ID:              adapterID,
		WorkspaceID:     &deps.workspace.ID,
		Name:            "SMTP Relay",
		AdapterType:     domain.AdapterTypeSMTP,
		ConfigEncrypted: []byte("encrypted"),
	}

	body := `{"config":{"host":"smtp.example.com","port":587,"tls_mode":"starttls","auth_mode":"plain","username":"apikey","password":"","from_email":"new@example.com"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/adapters/"+adapterID.String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := deps.echo.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(adapterID.String())
	deps.withWorkspace(c)

	if err := h.Update(&c); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	var encryptedConfig map[string]any
	if err := json.Unmarshal(deps.crypto.lastPlaintext, &encryptedConfig); err != nil {
		t.Fatalf("updated config JSON error = %v", err)
	}
	if encryptedConfig["password"] != "old-secret" {
		t.Fatalf("password = %v, want old-secret", encryptedConfig["password"])
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/http/handler -run TestAdapterHandler_Update_SMTPKeepsPasswordWhenBlank -count=1
```

Expected: FAIL because update replaces the password with an empty string.

- [ ] **Step 3: Add SMTP password merge helper**

Add this helper in `internal/http/handler/adapter.go`:

```go
func mergeSMTPPassword(existingRaw []byte, updated json.RawMessage) (json.RawMessage, error) {
	var updatedMap map[string]any
	if err := json.Unmarshal(updated, &updatedMap); err != nil {
		return nil, err
	}
	if password, ok := updatedMap["password"].(string); ok && password != "" {
		return updated, nil
	}
	var existingMap map[string]any
	if err := json.Unmarshal(existingRaw, &existingMap); err != nil {
		return nil, err
	}
	if password, ok := existingMap["password"].(string); ok && password != "" {
		updatedMap["password"] = password
	}
	merged, err := json.Marshal(updatedMap)
	if err != nil {
		return nil, err
	}
	return merged, nil
}
```

In the update path, after decrypting existing config and before validation/encryption, apply:

```go
updated := *req.Config
if adapter.AdapterType == domain.AdapterTypeSMTP {
	merged, err := mergeSMTPPassword(decrypted, updated)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid SMTP config")
	}
	updated = merged
}
```

- [ ] **Step 4: Run the update test**

Run:

```bash
go test ./internal/http/handler -run TestAdapterHandler_Update_SMTPKeepsPasswordWhenBlank -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/http/handler/adapter.go internal/http/handler/adapter_test.go
git commit -m "feat: preserve smtp password on update"
```

### Task 6: Resolve SMTP From Address and Manual Identity

**Files:**
- Modify: `internal/resolution/from_resolver.go`
- Modify: `internal/service/from_email.go`
- Modify: `internal/service/identity.go`
- Modify: `internal/http/handler/adapter.go`
- Test: `internal/service/test_send_test.go`
- Test: `internal/service/identity_factory_test.go` or `internal/service/identity_test.go`

- [ ] **Step 1: Add failing SMTP fallback test**

Add this test to `internal/service/test_send_test.go`:

```go
func TestTestSendService_FallsBackToSMTPFromEmailWhenNoDefaultIdentity(t *testing.T) {
	f := newTestSendFixture(t)
	f.adapter.AdapterType = domain.AdapterTypeSMTP
	f.adapter.ConfigEncrypted = []byte(`{"host":"localhost","port":1025,"tls_mode":"none","from_email":"smtp@example.com","from_name":"SMTP"}`)
	f.crypto.decryptFn = func(_ []byte) ([]byte, error) {
		return f.adapter.ConfigEncrypted, nil
	}
	f.identityStore.getDefaultFn = func(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
		return nil, domain.ErrNoDefaultIdentity
	}

	result, err := f.service.Send(context.Background(), &service.TestSendRequest{
		TemplateID: f.template.ID,
		To:         "recipient@example.com",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.FromAddress != "smtp@example.com" {
		t.Fatalf("FromAddress = %q, want smtp@example.com", result.FromAddress)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/service -run TestTestSendService_FallsBackToSMTPFromEmailWhenNoDefaultIdentity -count=1
```

Expected: FAIL because `ResolveFromAddress` only knows `delegate_email`.

- [ ] **Step 3: Add SMTP from fallback**

Update `internal/resolution/from_resolver.go` comments and logic:

```go
// ResolveFromAddress determines the sender email address for an adapter by trying:
// 1. The default identity from the identity store.
// 2. SMTP from_email from config_meta.
// 3. Gmail delegate_email from config_meta.
// 4. SMTP from_email or Gmail delegate_email from decrypted config JSON.
```

Add before delegate lookup:

```go
if fromEmail := adapter.ConfigMeta["from_email"]; fromEmail != "" {
	from.Address = fromEmail
	return from
}
```

In decrypted fallback, add:

```go
if smtpFrom, ok := cfgMap["from_email"].(string); ok && smtpFrom != "" {
	from.Address = smtpFrom
	if name, ok := cfgMap["from_name"].(string); ok && name != "" {
		from.Name = name
	}
	return from
}
```

Update `resolveFromEmail` in `internal/service/from_email.go` so SMTP can use the config fallback when called from real send. Change signature to:

```go
func resolveFromEmail(identityStore port.AdapterIdentityStore, ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte, senderIdentityID *uuid.UUID) (string, error)
```

When no explicit identity is set and default identity lookup fails:

```go
from := resolution.ResolveFromAddress(ctx, identityStore, adapter, decryptedConfig)
if from.Address != "" {
	return from.Address, nil
}
return "", fmt.Errorf("%w: adapter %s", domain.ErrNoDefaultIdentity, adapter.ID)
```

Update callers in `internal/service/send.go` and `internal/service/send_context.go` to pass decrypted config from the resolved adapter path if available. If the prepared send path does not have decrypted config, decrypt it before resolving from.

- [ ] **Step 4: Allow SMTP manual identity without provider domain**

In `internal/service/identity.go`, load the adapter at the beginning of `CreateManual`:

```go
adapter, err := s.adapterStore.GetByID(ctx, adapterID)
if err != nil {
	return nil, err
}
```

Wrap the domain-verified check:

```go
if adapter.AdapterType != domain.AdapterTypeSMTP {
	existing, err := s.identityStore.ListByAdapter(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	domainVerified := false
	for _, ident := range existing {
		if ident.IdentityType == domain.IdentityTypeDomain &&
			ident.Identity == emailDomain &&
			ident.Status == domain.IdentityStatusVerified {
			domainVerified = true
			break
		}
	}
	if !domainVerified {
		return nil, fmt.Errorf("%w: domain %s is not verified in this adapter", domain.ErrIdentityNotInDomain, emailDomain)
	}
}
```

- [ ] **Step 5: Seed SMTP default identity on create/update**

In `internal/http/handler/adapter.go`, add helper:

```go
func smtpIdentityFromConfig(raw []byte) (identity string, displayName *string, ok bool) {
	var cfg smtpadapter.Config
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.FromEmail == "" {
		return "", nil, false
	}
	if cfg.FromName != "" {
		name := cfg.FromName
		displayName = &name
	}
	return cfg.FromEmail, displayName, true
}
```

After successful create/update for SMTP, call `IdentityService.CreateManual` or a new small service helper that upserts the default SMTP identity. If the existing identity already exists, set it default through the store rather than creating a duplicate.

- [ ] **Step 6: Run service and handler tests**

Run:

```bash
go test ./internal/service -run 'TestTestSendService_FallsBackToSMTPFromEmail|TestIdentityService' -count=1
go test ./internal/http/handler -run 'TestAdapterHandler_Create_SMTP|TestAdapterHandler_Update_SMTP' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/resolution/from_resolver.go internal/service/from_email.go internal/service/identity.go internal/http/handler/adapter.go internal/service/test_send_test.go internal/http/handler/adapter_test.go
git commit -m "feat: resolve smtp sender identities"
```

### Task 7: Backend Contract Docs and Verification

**Files:**
- Modify: `docs/API.md`
- Modify: `docs/DEPLOYMENT.md`
- Modify: `docs/EMAIL_FLOWS.md`
- Modify: `docs/postman/senda-api-v1.postman_collection.json`

- [ ] **Step 1: Update docs with SMTP contract**

Add the contract JSON from this plan to the Adapters section in `docs/API.md` and the SMTP section in `docs/DEPLOYMENT.md`.

- [ ] **Step 2: Update Postman adapter type text**

Replace “Adapter types: ses, gmail” with “Adapter types: ses, gmail, smtp” in `docs/postman/senda-api-v1.postman_collection.json`.

- [ ] **Step 3: Run backend gate**

Run:

```bash
make lint
make vet
make test
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add docs/API.md docs/DEPLOYMENT.md docs/EMAIL_FLOWS.md docs/postman/senda-api-v1.postman_collection.json
git commit -m "docs: document smtp adapter contract"
```

---

## Issue 2: Frontend Adapters Module

### Task 8: Add SMTP Types

**Files:**
- Modify: `web/src/types/adapters.ts`
- Test: use TypeScript typecheck.

- [ ] **Step 1: Update adapter types**

Change `web/src/types/adapters.ts`:

```ts
export type AdapterType = "ses" | "gmail" | "smtp";
export type SmtpTLSMode = "none" | "starttls" | "implicit_tls";
```

Add:

```ts
export interface SmtpConfig {
  host: string;
  port: number;
  tls_mode: SmtpTLSMode;
  auth_mode?: "plain" | "login";
  username?: string;
  password?: string;
  from_email: string;
  from_name?: string;
}
```

Update request unions:

```ts
config: SesConfig | GmailConfig | SmtpConfig;
```

- [ ] **Step 2: Run typecheck**

Run:

```bash
corepack pnpm --dir web typecheck
```

Expected: TypeScript fails in components that assume only SES/Gmail.

- [ ] **Step 3: Commit after fixing only type definitions if typecheck still passes**

If typecheck passes after the type-only change:

```bash
git add web/src/types/adapters.ts
git commit -m "feat: add smtp adapter frontend types"
```

If it fails, continue to Task 9 and commit both tasks together.

### Task 9: Add SMTP Fields to Adapter Form

**Files:**
- Modify: `web/src/components/adapters/adapter-form.tsx`

- [ ] **Step 1: Add SMTP defaults**

Update `ADAPTER_DEFAULTS`:

```ts
smtp: {
  rateLimit: 10,
  description: "SMTP relay rate limits depend on your provider or internal relay policy.",
},
```

- [ ] **Step 2: Add SMTP state**

Add state near SES/Gmail state:

```tsx
const [smtpHost, setSmtpHost] = useState(defaults?.config_meta?.host ?? "");
const [smtpPort, setSmtpPort] = useState(defaults?.config_meta?.port ?? "587");
const [smtpTLSMode, setSmtpTLSMode] = useState<"none" | "starttls" | "implicit_tls">(
  (defaults?.config_meta?.tls_mode as "none" | "starttls" | "implicit_tls" | undefined) ?? "starttls"
);
const [smtpAuthMode, setSmtpAuthMode] = useState<"plain" | "login">("plain");
const [smtpUsername, setSmtpUsername] = useState("");
const [smtpPassword, setSmtpPassword] = useState("");
const [smtpFromEmail, setSmtpFromEmail] = useState(defaults?.config_meta?.from_email ?? "");
const [smtpFromName, setSmtpFromName] = useState("");
```

- [ ] **Step 3: Include SMTP in type selector**

Add:

```tsx
<SelectItem value="smtp">SMTP</SelectItem>
```

- [ ] **Step 4: Add SMTP payload construction**

In submit construction, build SMTP config:

```ts
const config =
  adapterType === "ses"
    ? { region: sesRegion, access_key_id: sesAccessKey, secret_access_key: sesSecretKey }
    : adapterType === "gmail"
      ? { service_account_json: gmailServiceAccountJSON, delegate_email: gmailDelegateEmail }
      : {
          host: smtpHost,
          port: Number(smtpPort),
          tls_mode: smtpTLSMode,
          ...(smtpUsername || smtpPassword ? { auth_mode: smtpAuthMode } : {}),
          ...(smtpUsername ? { username: smtpUsername } : {}),
          ...(smtpPassword ? { password: smtpPassword } : {}),
          from_email: smtpFromEmail,
          ...(smtpFromName ? { from_name: smtpFromName } : {}),
        };
```

- [ ] **Step 5: Render SMTP fields**

Refactor the config section to branch `ses`, `gmail`, `smtp`. Add this SMTP block:

```tsx
{adapterType === "smtp" && (
  <div className="flex flex-col gap-3">
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-host">Host</Label>
        <Input id="smtp-host" value={smtpHost} onChange={(e) => setSmtpHost(e.target.value)} placeholder="smtp.example.com" className="font-mono" required={!isEdit} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-port">Port</Label>
        <Input id="smtp-port" type="number" value={smtpPort} onChange={(e) => setSmtpPort(e.target.value)} placeholder="587" className="font-mono" required={!isEdit} />
      </div>
    </div>
    <div className="flex flex-col gap-2">
      <Label>Auth Mode</Label>
      <Select value={smtpAuthMode} onValueChange={(v) => setSmtpAuthMode(v as "plain" | "login")}>
        <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="plain">PLAIN</SelectItem>
          <SelectItem value="login">LOGIN</SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div className="flex flex-col gap-2">
      <Label>TLS Mode</Label>
      <Select value={smtpTLSMode} onValueChange={(v) => setSmtpTLSMode(v as "none" | "starttls" | "implicit_tls")}>
        <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="starttls">STARTTLS</SelectItem>
          <SelectItem value="implicit_tls">Implicit TLS</SelectItem>
          <SelectItem value="none">None</SelectItem>
        </SelectContent>
      </Select>
      {smtpTLSMode === "none" && (
        <p className="text-xs text-amber-400">Plain SMTP sends without transport encryption. Use only for Mailpit, local testing, or trusted internal relays.</p>
      )}
    </div>
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-username">Username</Label>
        <Input id="smtp-username" value={smtpUsername} onChange={(e) => setSmtpUsername(e.target.value)} placeholder="user or API key" className="font-mono" />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-password">Password</Label>
        <Input id="smtp-password" type="password" value={smtpPassword} onChange={(e) => setSmtpPassword(e.target.value)} placeholder={isEdit ? "Leave empty to keep current" : "secret"} className="font-mono" />
      </div>
    </div>
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-from-email">From Email</Label>
        <Input id="smtp-from-email" type="email" value={smtpFromEmail} onChange={(e) => setSmtpFromEmail(e.target.value)} placeholder="no-reply@example.com" className="font-mono" required={!isEdit} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="smtp-from-name">From Name</Label>
        <Input id="smtp-from-name" value={smtpFromName} onChange={(e) => setSmtpFromName(e.target.value)} placeholder="Senda" />
      </div>
    </div>
  </div>
)}
```

- [ ] **Step 6: Run frontend typecheck**

Run:

```bash
corepack pnpm --dir web typecheck
```

Expected: PASS after fixing branch exhaustiveness and submit readiness.

- [ ] **Step 7: Commit**

```bash
git add web/src/types/adapters.ts web/src/components/adapters/adapter-form.tsx
git commit -m "feat: add smtp adapter form"
```

### Task 10: Add SMTP Badge and Action Rules

**Files:**
- Modify: `web/src/components/adapters/adapter-type-badge.tsx`
- Modify: `web/src/components/adapters/adapters-content.tsx`

- [ ] **Step 1: Add SMTP badge**

In `adapter-type-badge.tsx`, add an SMTP icon:

```tsx
function SmtpIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" className={className}>
      <path d="M2 4h10v6H2z" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
      <path d="M4 2h6M4 12h6M7 2v2M7 10v2" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  );
}
```

Add config:

```tsx
smtp: {
  label: "SMTP",
  textColor: "text-cyan-300",
  bgColor: "bg-cyan-500/10",
  icon: <SmtpIcon className="h-3.5 w-3.5" />,
},
```

- [ ] **Step 2: Hide provider-only actions for SMTP**

In `adapters-content.tsx`, ensure checks are explicit:

```ts
const supportsIdentitySync = adapter.adapter_type === "ses" || adapter.adapter_type === "gmail";
const supportsProvisioning = adapter.adapter_type === "ses";
const supportsSenderSharing = adapter.adapter_type === "ses";
const supportsAdapterSharing = adapter.adapter_type === "gmail";
```

Use these booleans to render sync/provisioning/sharing actions. Do not render SES/Gmail actions for SMTP.

- [ ] **Step 3: Test send from SMTP**

Change test-send payload:

```ts
...(adapter.adapter_type === "ses" && from ? { from } : {}),
```

Keep this SES-only unless backend later supports choosing SMTP identities manually in the test dialog. SMTP should use the backend default `from_email`.

- [ ] **Step 4: Run frontend checks**

Run:

```bash
corepack pnpm --dir web typecheck
corepack pnpm --dir web lint
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/adapters/adapter-type-badge.tsx web/src/components/adapters/adapters-content.tsx
git commit -m "feat: show smtp adapters in ui"
```

### Task 11: Frontend Verification in Browser

**Files:**
- No source files unless visual QA finds issues.

- [ ] **Step 1: Start local app if it is not already running**

Run:

```bash
make dev
```

Expected: frontend available at `http://localhost:3000`.

- [ ] **Step 2: Open Adapters page**

Use the in-app browser to navigate to the tenant/workspace Adapters route for the local seeded workspace.

Expected: existing adapters list renders.

- [ ] **Step 3: Create SMTP authenticated plain adapter**

Use the UI:

```text
Type: SMTP
Name: Postal SMTP
Host: 127.0.0.1
Port: 2525
TLS Mode: None
Auth Mode: PLAIN or LOGIN
Username: use the relay username provided by the user
Password: use the relay password provided by the user
From Email: noreply-senda@tether.education
Rate Limit: 10
```

Expected: create succeeds and table shows SMTP badge.

- [ ] **Step 4: Send test email**

Use Test Send with:

```text
Recipient: reynaldo@tether.education
Subject: SMTP UI test
Body: <p>Hello from SMTP</p>
```

Expected: success toast. The user will validate whether the message arrives at `reynaldo@tether.education`.

- [ ] **Step 5: Commit visual fixes if needed**

If visual fixes are needed:

```bash
git add web/src/components/adapters
git commit -m "fix: polish smtp adapter ui"
```

### Task 12: Final Gates

**Files:**
- All touched files.

- [ ] **Step 1: Run backend gate**

Run:

```bash
make lint
make vet
make test
```

Expected: PASS.

- [ ] **Step 2: Run frontend gate**

Run:

```bash
corepack pnpm --dir web typecheck
corepack pnpm --dir web lint
```

Expected: PASS. Node may warn if local Node is not the package engine version; warnings are acceptable when commands pass.

- [ ] **Step 3: Inspect final diff**

Run:

```bash
git diff --stat
git diff --check
```

Expected: no whitespace errors; diff only touches SMTP-related files and docs.

- [ ] **Step 4: Final commit if any changes remain**

```bash
git status --short
git add .
git commit -m "feat: add smtp adapter support"
```

Skip the commit if all prior task commits already captured every change.

---

## Self-Review Notes

- Spec coverage: backend contract, DB enum, sender runtime, handler validation, password preservation, identity fallback, UI form, badge, action gating, test send, docs, and gates are covered.
- Placeholder scan: no unresolved placeholder markers are intentionally left. The one migration down file is intentionally irreversible because PostgreSQL enum value removal is unsafe after application.
- Type consistency: canonical names are `AdapterTypeSMTP`, `smtp`, `SmtpConfig`, `tls_mode`, `from_email`, and `implicit_tls`.
