
# Senda web frontend

## Development

Use `pnpm` only.

```bash
corepack enable
(cd web && corepack install)
pnpm --dir web install
pnpm --dir web dev
```

## Validation

```bash
pnpm --dir web test
pnpm --dir web typecheck
pnpm --dir web lint -- --max-warnings=0
```

`pnpm --dir web test` is the canonical frontend test entrypoint. The standard repo validation flow intentionally does not require a local `next build`.

If you need the full frontend gate exactly as CI runs it, use:

```bash
make ci-frontend
```

## Notes

- The dashboard is environment-aware for workspace runtime flows (`prod` / `test`).
- External embed routes live under `web/src/app/embed/` and rely on the external integration bootstrap/session APIs.
- Keep route generation and navigation helpers aligned with the environment model helpers under `web/src/lib/` and `web/src/hooks/`.
