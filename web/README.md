# Senda web frontend

## Development

Use pnpm only.

```bash
corepack enable
(cd web && corepack install)
pnpm --dir web install
pnpm --dir web dev
```

## Validation

```bash
pnpm --dir web typecheck
pnpm --dir web lint -- --max-warnings=0
```

The repo validation gate intentionally does not require a local `next build`.
