# Adapters & Identities

Adapters are the providers that physically deliver email (SES, Gmail, SMTP).
Identities are verified senders within an adapter. SES additionally has a
provisioning lifecycle for tracking (SNS topics + configuration set).

## Quick reference

### Adapters — workspace-scoped (`<ws>` and `<envWS>`)

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/adapters` | viewer+ | Cursor pagination. Includes shared adapters when `AdapterAccessService` is configured. |
| POST | `<ws>/adapters` | admin | `{name, adapter_type: "ses" \| "gmail", config, is_default?, rate_limit_per_second?}`. Only `ses` and `gmail` are accepted; any other value (including `smtp`) returns 422 `VALIDATION_ERROR`. Config encrypted at rest. |
| POST | `<ws>/adapters/validate-ses` | admin | `{region, access_key_id, secret_access_key, endpoint_url?}`. Verifies AWS creds without creating resources. 15s timeout. |
| GET | `<ws>/adapters/:id` | viewer+ | |
| PUT | `<ws>/adapters/:id` | admin | Patch parcial. `config` is merged, not replaced. 403 if shared and read-only. |
| DELETE | `<ws>/adapters/:id` | admin | Soft-delete; best-effort SES deprovision. |
| POST | `<ws>/adapters/:id/test` | viewer+ | `{to, subject, body, from?}`. Real send; 30s timeout. |
| GET | `<ws>/adapters/:id/workspace-access` | admin | **`_system` only**; otherwise 404. Lists workspace IDs with access. |
| PUT | `<ws>/adapters/:id/workspace-access` | admin | **`_system` only**. `{workspace_ids: [uuid]}` replaces (not merges). Audited. |
| GET | `<ws>/adapters/:id/setup-guide` | viewer+ | SES: tracking setup steps + IAM policy + webhook URL. Gmail: short message. |
| POST | `<ws>/adapters/:id/auto-provision-tracking` | admin | Auto-provisions SNS + SES configuration set. 501 if provisioner not configured. 422 with partial state on failure. |
| GET | `<ws>/adapters/:id/provisioning-status` | viewer+ | Returns each step (`not_started`, `in_progress`, `completed`, `failed`). |

### Identities — workspace-scoped

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/adapters/:id/identities` | viewer+ | SES email/domain identities accessible from this workspace. |
| POST | `<ws>/adapters/:id/identities` | admin | `{identity, display_name?}`. Manual entry. |
| POST | `<ws>/adapters/:id/identities/sync` | admin | Calls SES `ListEmailIdentities` and refreshes the table. |
| DELETE | `<ws>/adapters/:id/identities/:identity_id` | admin | |
| POST | `<ws>/adapters/:id/identities/:identity_id/set-default` | admin | Used by sends without explicit `from`. |
| GET | `<ws>/adapters/:id/identities/:identity_id/workspace-access` | admin | **`_system` only**. |
| PUT | `<ws>/adapters/:id/identities/:identity_id/workspace-access` | admin | **`_system` only**. `{workspace_ids: [uuid]}`. Audited. |

### Adapters & Identities — global

`/api/v1/manage/global/adapters[/:id][/...]` and the identities sub-resources.
RBAC = `superadmin`. Same operations, no `workspace-access`.

## Operational rules

### Provider semantics

- **SES**: full lifecycle (provision tracking, identities, sharing of email
  identities; domain identities are NOT shareable). Region required in
  config.
- **Gmail**: shared at the adapter level (whole adapter is granted to a
  workspace). No email identities sharing per item.

> **SMTP** is NOT exposed as an adapter type through the API. It exists as
> an internal sender used by dev stacks (Mailpit) and is configured via
> server config, not via `POST /adapters`. The handler rejects
> `adapter_type: "smtp"` with 422.

### Sharing

- Adapter-level sharing (`<ws>/adapters/:id/workspace-access`) and
  identity-level sharing (for SES) are scoped to the `_system` workspace —
  it is the control plane.
- Granting access to a child workspace makes the adapter / identity visible
  there as **read-only**. Mutation attempts from the child return
  403 `FORBIDDEN` with message "shared resource is read-only" (distinct
  from the injector signal `READ_ONLY_INHERITED_INJECTOR`). The divergence
  path for shared adapters is **not** "create a local same-name adapter"
  — that yields a separate adapter and breaks the sharing model. Instead,
  ask `_system` to revoke or modify the grant.
- Granting overwrites; the PUT replaces the list of workspace IDs.

### Provisioning steps (SES)

`auto-provision-tracking` runs the canonical 6-step provision:

1. Verify AWS credentials (uses validate-ses internally).
2. Create / locate SNS topic.
3. Create SES configuration set.
4. Bind tracking event types (delivery, bounce, complaint, open).
5. Subscribe Senda's `/api/v1/webhooks/ses/inbound` to the SNS topic.
6. Confirm subscription end-to-end.

Each step is recorded in `adapter_provisioning_steps`. On failure, the
endpoint returns 422 with the partial state; rerunning is idempotent and
resumes from the last failed step. Deprovisioning runs the inverse 4-step
sequence at `DELETE /adapters/:id`.

### Identity status

Each identity carries `verified` / `pending` / `failed`. Senda does not
re-verify; it reflects what the provider reports during sync. If the
adapter's default identity is not `verified`, sends fail with 422
`NO_DEFAULT_IDENTITY` or 422 `DOMAIN_NOT_VERIFIED`.

### Default identity & template type binding

Resolution order at send time, for choosing the sender:

1. `template_type.sender_identity_id` (if set).
2. Adapter's default identity (`set-default`).
3. Failure with `NO_DEFAULT_IDENTITY`.

`from_email` is no longer a template version field (column dropped in
migration 023); it is always derived from the resolved identity.

## Flujo end-to-end — bring up SES on a workspace

1. `POST <ws>/adapters/validate-ses` to verify creds (no AWS state yet).
2. `POST <ws>/adapters` with `adapter_type: "ses"` and the config.
3. `POST <ws>/adapters/:id/auto-provision-tracking`.
4. `GET <ws>/adapters/:id/provisioning-status` until all steps are
   `completed`.
5. `POST <ws>/adapters/:id/identities/sync`.
6. `POST <ws>/adapters/:id/identities/:identity_id/set-default` once the
   intended identity is `verified`.
7. Bind `template_type.adapter_id` to this adapter (and optionally
   `sender_identity_id`).

## Sharing flow (`_system` adapter to child workspaces)

1. Create the adapter / identity in `_system`.
2. From `_system`: `PUT <ws>/adapters/:id/workspace-access` with the list of
   workspace UUIDs that should see it.
3. Children now see it via their `GET .../adapters` and can use it for
   sending; modifications still belong to `_system`.

## Cuándo consultar OpenAPI / MCP

- Adapter `config` shape per `adapter_type` (`ses`, `gmail`).
- `auto-provision-tracking` partial-error body for retry decisions.
- `setup-guide` exact step list (changes per AWS region).

## Gotchas

- `validate-ses` does not create resources; it just probes credentials.
- `auto-provision-tracking` is **not** a no-op on partial state; it resumes,
  and idempotent steps may still take seconds.
- DELETE adapter performs **best-effort** SES deprovision; if AWS fails, the
  adapter is still soft-deleted but the AWS side may need manual cleanup.
- Workspace-access endpoints are paths under any workspace, but they only
  do something when the called workspace is the tenant's `_system`. Other
  workspaces return 404.
- The SES provider webhook (`/api/v1/webhooks/ses/inbound`) verifies SNS
  signatures and uses an anti-replay store. See `webhooks-and-events.md`.
- Test sends from the adapter (`POST .../adapters/:id/test`) bypass the
  template engine. They are useful for validating connectivity but do NOT
  validate that templates render.
