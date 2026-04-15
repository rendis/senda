# Verify Report — Autonomous E2E Isolation

## Resultado

- **Estado técnico**: implementación y verificación finalizadas
- **Estado SDD**: `done`
- **Resultado E2E autónomo**: **OK**
- **Reviewer final**: `James` → **APPROVED**

## Hallazgos

1. El scope canónico compartido en `internal/teststack` preserva `spec`, `worktree`, `mode` y `run`, con hash estable y nombres Docker-safe.
2. `RuntimeReport.ArtifactDir` queda alineado con el artifact root real del runner usando `filepath.Dir(opts.OutPath)`.
3. `Down()` conserva el `env-report.json` y exige identidad suficiente del report antes de limpiar recursos.
4. El smoke `test/system/smoke-dual-stack.sh` tenía un fallo de shell real: `jq | while read` bajo `set -euo pipefail` devolvía error al llegar a EOF; eso se corrigió con process substitution.

## Verificaciones ejecutadas

### 1) Unit / compile-targeted

```bash
go test ./internal/teststack ./cmd/systemtest
```

Resultado:

- `internal/teststack` → **OK**
- `cmd/systemtest` → **OK** (`[no test files]`, compilación válida)

### 2) Compilación dirigida del paquete E2E sin levantar harness interno

```bash
SENDA_E2E_EXTERNAL_STACK=1 MAILPIT_URL=http://127.0.0.1 SENDA_BASE_URL=http://127.0.0.1 SENDA_DATABASE_URL=postgres://127.0.0.1/senda go test -tags=e2e ./test/e2e -run 'TestUseExternalStackEnv|TestTruthyEnv'
```

Resultado:

- `test/e2e` → **OK**

### 3) Smoke concurrente dual-stack

```bash
tmp_docker_config=$(mktemp -d)
printf '{}' > "$tmp_docker_config/config.json"
DOCKER_CONFIG="$tmp_docker_config" ./test/system/smoke-dual-stack.sh
```

Resultado:

- **OK**
- Salida final: `dual-stack smoke ok`
- Se verificó:
  - `runtime.artifact_dir` coincide con el artifact root de cada run
  - `runtime.network` es distinto en ambos runs
  - `runtime.containers.*` es distinto en ambos runs
  - health de `senda`, `mailpit` y `keycloak` en ambos stacks
  - cleanup final sin redes ni contenedores residuales

## Estado técnico final

El change **cumple DoD**.  
No quedan fallos técnicos abiertos dentro del alcance de `autonomous-e2e-isolation`.

## Signoff final

James aprobó el cierre con estos puntos:

1. La colisión real de recursos quedó resuelta: el harness preserva modo `e2e` en el scope canónico y el stack sigue usando nombres Docker/network derivados del scope.
2. El contrato de artefactos ahora es coherente y verificable: `runtime.artifact_dir` coincide con el root real del run y `Down()` preserva `env-report.json`.
3. El smoke concurrente valida `artifact_dir`, redes y contenedores distintos, y además comprueba cleanup efectivo sin residuos.
4. El runner conserva el camino determinístico por defecto y deja visual/a11y/chaos fuera salvo activación explícita, alineado con el spec.
