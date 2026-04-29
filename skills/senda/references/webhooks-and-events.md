# Webhooks & Events

Senda has three webhook surfaces:

1. **Outbound webhooks** — Senda → your endpoint, signed with HMAC, on
   send/email events.
2. **Inbound provider webhook** — SES → SNS → Senda (`/api/v1/webhooks/ses/inbound`).
3. **Tracking pixel** — recipient browser → `/t/o/:tracking_id` for opens.

## Outbound webhooks (workspace-scoped)

### Endpoints (`<ws>` and `<envWS>`)

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/webhooks` | workspace_admin | `{url, events: [string]}`. `url` must be HTTPS to a public host (private/loopback IPs rejected). The HMAC `secret` is generated server-side, returned ONCE in the response. |
| GET | `<ws>/webhooks` | workspace_viewer+ | Cursor pagination. `secret` never returned. |
| GET | `<ws>/webhooks/:id` | workspace_viewer+ | |
| PUT | `<ws>/webhooks/:id` | workspace_admin | Patch `{url?, events?, is_active?}`. `secret` is not rotated by PUT. |
| DELETE | `<ws>/webhooks/:id` | workspace_admin | Hard delete (no soft-delete). |
| POST | `<ws>/webhooks/:id/test` | workspace_admin | Dispatches a `webhook.test` event with `{message, webhook_id}`. Returns `{"status": "dispatched"}`. |

### Outbound headers

Each outbound HTTP call carries:

```
X-Senda-Event:     <event_type>             # e.g. "email.delivered"
X-Senda-Timestamp: <unix_seconds>
X-Senda-Signature: sha256=<hex of HMAC-SHA256(secret, "<timestamp>.<raw_body>")>
```

`X-Senda-Event` is the canonical way to route by topic without parsing the
body. The HMAC input for `X-Senda-Signature` is the timestamp, a literal
`.` separator, and the raw body — in that order.

Reject the request server-side if:

- the `sha256=` prefix is missing,
- the recomputed HMAC does not match (use constant-time compare),
- the timestamp is too old (recommended: ±5 minutes from now).

### Event topics

Outbound webhooks are dispatched by `EventProcessor`, which is wired only
to the **provider inbound path** (SES → SNS). The events that actually fire
deliveries are therefore the provider lifecycle events:

- `email.delivered`
- `email.bounced`
- `email.complained`
- `email.opened`

(Plus `webhook.test` from `POST /webhooks/:id/test`.)

Internal lifecycle states like `queued`, `processing`, `sent`, `failed`,
`suppressed` are persisted as email events but **do not currently dispatch
outbound webhooks**. Subscribing to them is harmless but receives nothing.
Track those states via the management API (`GET <ws>/emails/:tracking_id/events`).

### Operational rules

- HTTPS is required. The check rejects private IP ranges and loopback.
- `secret` is shown only on creation. To rotate, delete and recreate.
- `is_active = false` keeps the row but suspends delivery.
- Retries are server-side; failed deliveries appear in audit log entries.
- `POST .../test` is meant for end-to-end verification; the body has fixed
  shape `{message: "...", webhook_id: "..."}`.

## Inbound provider webhook — SES

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/webhooks/ses/inbound` | None (verifies SNS signature) | Accepts SNS `SubscriptionConfirmation` and `Notification` messages. Body max 256KB. Verifies signature against AWS public certs. Validates `TopicArn` against the registered allowlist. Anti-replay store. Auto-confirms subscriptions. |

What the handler does with messages:

- `SubscriptionConfirmation` → fetches `SubscribeURL` and confirms (auto).
- `Notification` of a delivery / bounce / complaint / open → updates the
  email + emits the corresponding outbound webhook event.

### Operational rules

- The endpoint is **public** (no token). Security is the SNS signature plus
  TopicArn allowlist plus replay window.
- The TopicArn allowlist is populated when an SES adapter goes through
  `auto-provision-tracking` (see `adapters-and-identities.md`).
- 200 = accepted. 400 = malformed. 403 = signature/allowlist mismatch.

## Tracking pixel — opens

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/t/o/:tracking_id` | None | Returns a 1×1 transparent GIF. Records an `email.opened` event asynchronously. Dedup window 30s. Cache headers prevent caching. |

## Media proxy — video thumbnails

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/public/video-thumbnail?url=<thumbnail_url>` | None | SSRF-protected (no private IPs). Allowlist of hosts (`img.youtube.com`, `i.ytimg.com` by default). Composes a play-button overlay and returns PNG. 24h in-memory cache, max 500 entries. |

Used by templates with the `video` block in the visual builder; not part of
the regular send / event flow.

## Flujo end-to-end — wire up outbound delivery events

1. `POST <ws>/webhooks` with `events = ["email.delivered", "email.bounced",
   "email.complained", "email.opened"]`. **Save the `secret`.**
2. Implement an HTTPS endpoint that:
   - Reads `X-Senda-Event` to dispatch by topic.
   - Reads `X-Senda-Timestamp`, ensures it is within ±5 minutes of now.
   - Verifies `X-Senda-Signature` (`sha256=<hex>`) against
     `"<timestamp>.<raw_body>"` using HMAC-SHA256 with the secret.
3. `POST <ws>/webhooks/:id/test` to confirm reception.
4. Send a test email via the data plane; observe the events.

## Cuándo consultar OpenAPI / MCP

- The exact event payload schema per topic.
- The full list of event topics enabled in your build.
- The SES inbound endpoint expected message envelope.

## Gotchas

- The HMAC signature covers `"<X-Senda-Timestamp>.<raw_body>"` as received,
  prefixed with `sha256=`. Do not parse and re-stringify the body, and do not
  forget the literal `.` between timestamp and body.
- HTTPS-public-host check rejects `localhost` and 10/8, 172.16/12, 192.168/16.
- The SES inbound endpoint is shared across all SES adapters in the
  deployment. The TopicArn allowlist is what isolates them per workspace.
- Tracking pixel records open events even if the user opened the email
  multiple times within 30s — only one event is recorded per dedup window.
- The 200/400/403 split for SES inbound matters: 200 stops SNS retries even
  on internal failures; the handler is conservative about returning 200.
- Webhook `is_active = false` is the only soft-stop; DELETE is destructive.
