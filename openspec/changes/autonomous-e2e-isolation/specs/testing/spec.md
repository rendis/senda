# Delta para testing

## ADDED Requirements

### Requirement: Scope-based isolation

El sistema MUST derivar un scope único por spec/worktree/run y usarlo para nombres de red, contenedores y rutas de artefactos.

#### Scenario: Two stacks in parallel

- GIVEN dos ejecuciones con spec o worktree distintos
- WHEN ambas levantan E2E/system al mismo tiempo
- THEN the two runs MUST NOT share network or container names
- AND cada una MUST write to its own artifact root

#### Scenario: Same host, same mode

- GIVEN dos runs del mismo modo sobre el mismo host
- WHEN el scope cambia
- THEN los recursos MUST remain isolated

### Requirement: Guaranteed cleanup

El sistema MUST ejecutar cleanup usando el mismo scope que creó los recursos, incluso si una etapa falla.

#### Scenario: Failure during a stage

- GIVEN una etapa falla a mitad del run
- WHEN termina el runner
- THEN the cleanup MUST run
- AND los artefactos generados MUST remain available

### Requirement: Deterministic default path

El runner MUST ejecutar por defecto solo las etapas determinísticas base.
Las etapas visual, a11y y chaos MUST stay out of the default path and MAY run only with explicit activation.

#### Scenario: Default PR run

- GIVEN un run estándar
- WHEN no se habilitan flags extra
- THEN visual, a11y y chaos MUST be skipped

#### Scenario: Explicit heavy run

- GIVEN flags explícitos para heavy stages
- WHEN el runner inicia
- THEN those stages MUST run

### Requirement: Consistent artifacts

El sistema MUST producir `env-report`, `stage-results`, logs, junit y `run-result` con rutas coherentes por scope.

#### Scenario: Successful run

- GIVEN un run completo
- WHEN finaliza
- THEN the artifact set MUST exist
- AND each file MUST be traceable to the same scope

### Requirement: Autonomous smoke validation

El sistema MUST prove that two isolated stacks can run simultaneously as the final validation gate for this change.

#### Scenario: Dual-stack smoke

- GIVEN two isolated scopes on the same machine
- WHEN both runs execute together
- THEN both MUST complete without resource collision
- AND both MUST clean up their own resources
