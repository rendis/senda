
# Resolution and Inheritance

This reference explains how Senda resolves configuration across scopes and environments.

## Scope chain

For most workspace runtime resolution, the chain is:

1. workspace
2. tenant `_system` workspace
3. global

The first applicable match wins unless the resource type explicitly merges field-by-field.

## `_system` workspace

Each tenant has a special `_system` workspace.

Purpose:

- tenant-wide defaults
- selective sharing
- parent for workspace inheritance

Do not treat `_system` as a normal business workspace.

## Owned vs inherited vs shared

Agents must distinguish three states:

- **owned** — created directly in the current scope
- **inherited** — resolved from a parent scope by normal hierarchy
- **shared** — exposed intentionally from `_system` to a child workspace

These are not interchangeable.

## Shared defaults and forks

Recent template/workspace behavior includes:

- `_system` defaults visible to child workspaces
- explicit forks where a child workspace copies inherited content to own it locally
- exact version cloning for template versions

When working with templates:

- use forking when a child workspace needs its own copy of inherited behavior
- use exact version cloning when you need a new draft that exactly copies one version and its locales

## Selective sharing

### Gmail

Shared at the adapter level from tenant `_system` to selected workspaces.

### SES

Shared at the **email identity** level, not entire domain identities.

Rules:

- domain identities are not shareable
- child workspaces can only use SES email identities explicitly granted to them
- shared resources appear as read-only in child workspaces

## Injector and template precedence

### Injectors

- DB injectors resolve through scope chain
- code injectors merge into the same namespace
- request-body injector overrides may apply per field according to runtime policy

### Templates

- template types and templates resolve through visible scope
- versions/locales belong to the resolved template
- exact version cloning creates a new draft copy of a source version and all its locales

## Agent rules

- Do not collapse inherited and shared into the same concept.
- When a resource is inherited/shared, verify whether mutation is allowed before calling update/delete actions.
- When using `_system`, remember it is the control point for tenant-wide defaults and sharing policies.
