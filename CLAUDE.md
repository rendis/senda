# Senda — Claude Code Entry Point

## Project Overview

Senda is an open-source email orchestration platform built with Go + PostgreSQL (no Redis). It implements a 3-level hierarchy (Global → Tenant → Workspace) with inheritance chain resolution for templates, injectors, adapters, and domains.

**Stack:** Go 1.22+, PostgreSQL 16 + pg_cron, Echo v5, River (queue), pgx v5, gomjml, go-msgauth, golang-migrate

**Architecture:** Hexagonal (Ports & Adapters), TDD mandatory, TestContainers for integration tests.

---

## Session Workflow (cómo trabajar conmigo)

### Al iniciar cada sesión:

1. **Lee este archivo** (`CLAUDE.md`) — ya lo estás haciendo
2. **Revisa el estado actual:**
   ```bash
   ls stories/in-progress/    # ¿Hay algo a medio hacer?
   ls stories/done/            # ¿Qué ya está completado?
   ```
3. **Si hay una HT in-progress** → continúa donde se quedó (lee la sección "Log de Progreso" de esa HT)
4. **Si no hay nada in-progress** → consulta `stories/MANIFEST.md` sección "Ready to Start" y elige la siguiente según el orden recomendado

### Para implementar una HT:

1. Lee la HT completa (`stories/backlog/HT-XX.md`)
2. Lee las secciones del TECH_SPEC referenciadas en `spec_sections`
3. Mueve la HT a `stories/in-progress/` y actualiza su front matter
4. Implementa con TDD (test → code → refactor)
5. Registra decisiones y avances en la HT ("Notas de Implementación" y "Log de Progreso")
6. Cuando todos los criterios de aceptación estén cumplidos:
   - Ejecuta quality gates (`make test`, `make lint`, `go vet`)
   - Mueve la HT a `stories/done/`
   - Actualiza `stories/MANIFEST.md` (status, counters, "Ready to Start")

### Si la sesión se corta a mitad de una HT:

- La HT queda en `stories/in-progress/` con su "Log de Progreso" actualizado
- La siguiente sesión retoma desde ahí — no repitas trabajo ya hecho

### Prompt tipo para iniciar sesión:

```
Revisa el estado del proyecto y continúa con la siguiente HT disponible.
```

O si querés ser específico:

```
Implementa HT-01. Lee la story y las secciones §9 y §18 del TECH_SPEC.
```

---

## Teams de Trabajo (paralelización)

El proyecto se ejecuta con **equipos especializados** que trabajan en paralelo. Cada team es una sesión de Claude Code con un perfil y contexto optimizado para su track.

### Team Definitions

**Team Infra** — Track A (Foundation + Infrastructure)

- **Perfil:** DevOps / Platform Engineer
- **Expertise:** Docker, PostgreSQL, migrations, crypto, caching, rate limiting
- **HTs:** HT-01 → HT-02 → HT-03 → HT-04 → HT-13 → HT-14
- **Spec focus:** §3, §4, §5, §7, §9, §17, §18, §22, §23, §24

**Team Domain** — Track B (Core Domain + Resolution Engine)

- **Perfil:** Domain Engineer / DDD Specialist
- **Expertise:** Domain modeling, hexagonal architecture, ports & adapters, resolution algorithms
- **HTs:** HT-05 → HT-06 → HT-07 → HT-08 → HT-09 → HT-10 → HT-11 → HT-12
- **Spec focus:** §10, §11, §12, §6

**Team API** — Track C (API Layer + Auth)

- **Perfil:** Backend API Engineer
- **Expertise:** HTTP handlers, middleware, OIDC/JWT, RBAC, REST API design
- **HTs:** HT-17 → HT-18 → HT-19 → HT-20 → HT-21 → HT-27 → HT-25
- **Spec focus:** §8, §14, §15, §20

**Team SendOps** — Track D (Send Flow + Operations)

- **Perfil:** Integration / Systems Engineer
- **Expertise:** Message queues, workers, webhooks, provider integrations, observability
- **HTs:** HT-15 → HT-16 → HT-22 → HT-23 → HT-24 → HT-26
- **Spec focus:** §13, §16, §19, §21

**Team Frontend** — Track E (Frontend + Design-to-Code)

- **Perfil:** Frontend Engineer / Design Systems Specialist
- **Expertise:** Next.js, TypeScript, Tailwind CSS, React, component architecture, Pencil MCP
- **HTs:** HT-28 → HT-29 → HT-30 → HT-31 → HT-32 → HT-33 → HT-34 → HT-35 → HT-36
- **Spec focus:** DESIGN_BRIEF (§3 a §8), PRD (§5 User Stories US-36 a US-45)
- **Stack:** Next.js 16, TypeScript 5, Tailwind v4, shadcn/ui, TanStack Query 5, TanStack Table 9, React Hook Form 7, Zod 4, Auth.js v5, ky, Monaco Editor, Lucide React, Sileo
- **Diseño:** `senda_desing.pen` — leer SIEMPRE via Pencil MCP, NUNCA parsear el .pen como JSON
- **Flujo:** Pencil MCP lee frame → Claude genera componente React + Tailwind → verificar pixel-perfect → iterar en Pencil si hay drift
- **Bloqueado por:** HT-37 (QA Gate) — el frontend NO comienza hasta que el backend esté 100% testeado

**Team QA** — Track F (Quality Assurance + Security)

- **Perfil:** QA Engineer / Security Tester / Pentester
- **Expertise:** E2E testing, API testing, fuzzing, OWASP Top 10, race conditions, load testing, Postman, Go testing, TestContainers
- **HTs:** HT-37
- **Spec focus:** §6, §14, §15, §19, §20, §21, §24
- **Mentalidad:** Adversarial — buscar romper el sistema, no confirmar que funciona
- **Infraestructura:** TestContainers (PostgreSQL 16 + Mailpit + Senda Server + River workers)
- **Entregables:** E2E test suite, pentesting OWASP, chaos tests, colección Postman, reportes de cobertura y findings
- **Gate:** El frontend (Track E) NO comienza hasta que HT-37 esté en `done/`

### Parallelization Matrix

```
Semana   Team Infra        Team Domain         Team API          Team SendOps
─────────────────────────────────────────────────────────────────────────────
S1       HT-01             HT-05               —                 —
S2       HT-02 + HT-04     HT-06               —                 —
S3       HT-03             HT-07               HT-17             —
S4       HT-13 + HT-14     HT-08               HT-18             —
S5       ✅ done            HT-09               HT-19             —
S6       —                 HT-10               HT-20 + HT-21     —
S7       —                 HT-11               HT-27 + HT-25     —
S8       —                 HT-12               ✅ done            HT-15
S9       —                 ✅ done              —                 HT-16
S10      —                 —                   —                 HT-22 + HT-23
S11      —                 —                   —                 HT-24 + HT-26

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BACKEND COMPLETE ↑   |   QA GATE ↓   |   FRONTEND ↓↓
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Semana   Team QA
─────────────────────────
S12      HT-37 (E2E QA + Pentesting + Postman — espera TODO el backend)

Semana   Team Frontend (después del QA Gate)
─────────────────────────
S13      HT-28 (scaffolding + Pencil MCP + Design System)
S14      HT-29 (auth + scope switcher)
S15      HT-30 (onboarding wizard)
S15      HT-31 (dashboard + métricas)
S16      HT-33 (templates)
S16      HT-34 (injectors + adapters + domains)
S17      HT-36 (audit log + settings + empty states)
S17      HT-32 (emails)
S18      HT-35 (webhooks + API keys + members)
```

### Reglas de Paralelización

1. **HT-01 es bloqueante global** — todos los teams esperan a que termine (scaffolding del proyecto)
2. **Team Infra y Team Domain arrancan juntos** desde S1 (HT-01 y HT-05 solo dependen de HT-01)
3. **Team API arranca en S3** — necesita HT-02 (config) para HT-17 (Echo server)
4. **Team SendOps arranca en S8** — necesita resolvers (HT-10..12) + infra (HT-13, HT-14)
5. **Team QA arranca en S12** — necesita TODO el backend completado (Tracks A+B+C+D). HT-37 es bloqueante para el frontend
6. **Team Frontend arranca en S13** — después del QA Gate (HT-37 en `done/`). El frontend NO comienza hasta que el backend esté 100% testeado y cerrado
7. **Cross-team dependencies:** cuando una HT depende de otra de otro team, verificar que está en `stories/done/` antes de empezar
8. **Cada team mantiene su propia sesión** — el MANIFEST.md es el punto de sincronización compartido
9. **Pencil MCP obligatorio para Team Frontend** — leer diseño SIEMPRE via MCP server de Pencil, NUNCA parsear el .pen como JSON directo
10. **Pipeline:** `Backend (Tracks A+B+C+D) → QA Gate (Track F) → Frontend (Track E)`

### Prompt tipo para iniciar un team:

```
Sos el Team Infra. Tu perfil es DevOps/Platform Engineer.
Lee CLAUDE.md, revisa stories/done/ y stories/in-progress/,
y continúa con la siguiente HT de tu track (A).
Solo trabajá en HTs asignadas a tu team.
```

```
Sos el Team QA. Tu perfil es QA Engineer / Pentester.
Lee CLAUDE.md y HT-37. Verificá que TODOS los backend HTs estén en done/.
Tu trabajo es romper el sistema, no confirmar que funciona.
Levantá el stack E2E completo y ejecutá la batería de tests.
```

---

## Documentation Map

Documentation lives in `docs/`:

| Document                           | Purpose                                              | Read When                                |
| ---------------------------------- | ---------------------------------------------------- | ---------------------------------------- |
| `docs/specs/PRD_v5.md`             | Product requirements, user stories, business rules   | Understanding "why" and "what"           |
| `docs/specs/TECH_SPEC_v1.md`       | Complete technical specification (v1.4, ~5000 lines) | **Primary reference for implementation** |
| `docs/specs/TECH_STORIES.md`       | All 27 HTs with dependency graph and timeline        | Understanding scope and order            |
| `docs/specs/TESTING_STRATEGY.md`   | Test pyramid, patterns, coverage targets             | Writing tests                            |
| `docs/specs/SECURITY_CHECKLIST.md` | OWASP mapping, encryption, auth requirements         | Security-sensitive code                  |
| `docs/specs/DESIGN_BRIEF.md`       | UX/UI specification for frontend (future)            | Not needed for backend                   |

> Historical versions (PRD v1–v4, INITIAL_SPECT) live in `docs/archive/`.

### UI/UX Design — Pencil MCP (OBLIGATORIO)

El diseño base de la aplicación está en `senda_desing.pen` (raíz del proyecto), creado con [Pencil](https://www.pencil.dev/). Documentación oficial: https://docs.pencil.dev/

**REGLA: SIEMPRE usar Pencil MCP para interactuar con el diseño. NUNCA parsear el .pen como JSON.**

Pencil expone un MCP server local que corre cuando Pencil está abierto. Claude Code se conecta automáticamente y tiene acceso a todas las herramientas.

#### Herramientas MCP disponibles (USAR TODAS)

**Diseño — `batch_design`:**
- Crear, modificar, manipular elementos de diseño
- Operaciones: insert, copy, update, replace, move, delete
- Generar y colocar imágenes
- **Uso obligatorio** para cualquier modificación al .pen

**Lectura — `batch_get`:**
- Leer componentes y jerarquía del diseño
- Buscar elementos por patrones
- Inspeccionar estructura de componentes
- **Uso obligatorio** antes de implementar cualquier pantalla (leer el frame primero)

**Screenshots — `get_screenshot`:**
- Renderizar previews del diseño desde Pencil
- **Uso obligatorio** para Gate 1 del DoD: comparar screenshot de la app vs screenshot de Pencil

**Layout — `snapshot_layout`:**
- Analizar estructura del layout
- Detectar problemas de posicionamiento
- Encontrar elementos superpuestos
- **Usar** para verificar pixel-perfect después de implementar

**Editor — `get_editor_state`:**
- Contexto actual del editor
- Información de selección
- Detalles del archivo activo

**Variables — `get_variables` / `set_variables`:**
- Leer design tokens (colores, spacing, typography)
- Actualizar valores de tema
- Sincronizar con CSS/Tailwind
- **Uso obligatorio** para extraer tokens del Design System y mapearlos a Tailwind config

#### Flujo de trabajo con Pencil MCP

```
1. LEER diseño:
   batch_get → leer frame de la pantalla a implementar
   get_variables → extraer tokens (colores, spacing, typography)
   get_screenshot → capturar imagen de referencia del diseño

2. IMPLEMENTAR:
   Generar componentes React + Tailwind alineados al diseño
   Usar tokens extraídos en paso 1 para colores/spacing exactos

3. VALIDAR (Gate 1 — DoD):
   get_screenshot → capturar diseño de Pencil
   Comparar vs screenshot de la app corriendo (npm run dev)
   snapshot_layout → verificar estructura y posicionamiento
   Si hay drift → corregir en CÓDIGO, no en Pencil

4. ITERAR si hay problemas:
   batch_design → ajustar diseño solo si el diseñador lo aprueba
   set_variables → sincronizar tokens si cambiaron
```

#### Operaciones avanzadas

**Batch operations** para consistencia:
```
"Verificar que todos los botones usan la variable de color primario"
"Actualizar todos los headings para usar la escala tipográfica"
"Aplicar grid de 8px a todos los elementos"
```

**Sincronización código ↔ diseño:**
```
"Importar el Design System desde el Tailwind config a Pencil"
"Actualizar componentes React para matchear los diseños de Pencil"
"Sincronizar variables de tipografía entre CSS y Pencil"
```

**Generación de código desde diseño:**
```
"Generar código React para este componente"
"Crear Tailwind config desde estas variables de Pencil"
```

#### Troubleshooting

- **MCP no conecta:** Verificar que Pencil esté corriendo y el .pen abierto
- **Herramientas no aparecen:** Reiniciar Pencil y Claude Code
- **Cambios inesperados:** Ser más específico en prompts, pedir explicación antes de aplicar

---

## How Stories Work

### Directory Structure

```
senda/
├── CLAUDE.md              ← You are here (entry point)
├── stories/
│   ├── MANIFEST.md        ← Dependency graph + status overview
│   ├── backlog/           ← Stories not yet started
│   │   ├── HT-01.md
│   │   ├── HT-02.md
│   │   └── ...
│   ├── in-progress/       ← Currently being worked on
│   ├── done/              ← Completed stories
│   └── blocked/           ← Stories blocked by external dependency
├── docs/
│   ├── specs/             ← Docs vigentes (PRD, Tech Spec, etc.)
│   └── archive/           ← Versiones históricas
└── ...
```

### Story Lifecycle

1. **Pick a story** from `backlog/` — check MANIFEST.md for dependencies
2. **Move it** to `in-progress/` — update front matter `status: in-progress`
3. **Implement** following TDD: write test → make it pass → refactor
4. **Document** decisions in "Notas de Implementación" section
5. **Log progress** in "Log de Progreso" section
6. **Move to `done/`** when all acceptance criteria are met — update `status: done`

### Moving a Story

```bash
# Start working on HT-01
mv stories/backlog/HT-01.md stories/in-progress/
# Update status in front matter: status: in-progress

# Complete HT-01
mv stories/in-progress/HT-01.md stories/done/
# Update status in front matter: status: done

# Block a story
mv stories/in-progress/HT-03.md stories/blocked/
# Update status in front matter: status: blocked
# Add reason in "Notas de Implementación"
```

---

## Picking the Next Story

### Rules

1. **Never start a story whose dependencies aren't in `done/`**
2. **Prefer stories on the critical path** (Track A or B first)
3. **Only one story in-progress at a time** (per developer)
4. **Read the relevant TECH_SPEC sections** before writing any code

### Recommended Start Order (Solo Developer)

```
HT-01 → HT-05 → HT-02 → HT-06 → HT-03 → HT-04 → HT-07 → HT-14 →
HT-08 → HT-09 → HT-17 → HT-13 → HT-10 → HT-18 → HT-11 → HT-12 →
HT-15 → HT-16 → HT-19 → HT-20 → HT-21 → HT-25 → HT-27 → HT-22 →
HT-23 → HT-24 → HT-26
```

### Quick Dependency Check

Before starting HT-XX, verify:

```bash
# List completed stories
ls stories/done/

# Check required dependencies in the story's front matter
grep "dependencies:" stories/backlog/HT-XX.md
```

---

## Implementation Protocol

### For Each Story:

1. **Read the story file** completely (objective, deliverables, acceptance criteria)
2. **Read the referenced TECH_SPEC sections** (listed in `spec_sections` front matter)
3. **Write tests first** (TDD — Red → Green → Refactor)
4. **Follow project conventions:**
   - Go module: `github.com/senda-app/senda`
   - Folder structure per §9 of TECH_SPEC
   - Interfaces in `internal/port/`
   - Domain models in `internal/domain/`
   - Implementations in `internal/adapter/`
   - Services in `internal/service/`
   - HTTP handlers in `internal/http/handler/`
   - Middleware in `internal/http/middleware/`
5. **Use manual mocks** (no mock frameworks) — see TESTING_STRATEGY.md
6. **Integration tests** use TestContainers (tagged `//go:build integration`)
7. **Update the story** with implementation notes and progress log
8. **Run all tests** before marking done

### Code Quality Gates

```bash
make test                 # Unit tests pass
make test-integration     # Integration tests pass (if applicable)
make lint                 # golangci-lint passes
go vet ./...              # No issues
```

---

## Reglas de Código Fundamentales

### Regla #0: NUNCA iterar sobre código roto

**Esta es la regla más importante del proyecto.** No se puede trabajar sobre error ni iterar sobre código mal hecho. Si algo está mal, la respuesta correcta es:

1. **PARAR** — no intentar "hacer que funcione" con parches
2. **DIAGNOSTICAR** — entender la causa raíz, no el síntoma
3. **REFACTORIZAR** — corregir el diseño/approach, no parchear el error
4. **REIMPLEMENTAR** — si el approach es fundamentalmente incorrecto, reescribir desde cero

**Anti-patrones prohibidos:**
- Agregar `if err != nil { // ignore }` para "saltar" un error
- Wrappear código roto en try/catch o recover para que "no falle"
- Copiar-pegar código que no se entiende completamente
- Agregar flags/booleans para "desactivar" la parte que falla
- Cambiar tests para que pasen con el comportamiento incorrecto
- Acumular TODO/FIXME sin resolverlos antes de marcar done

**Lo correcto:**
- Si un test falla → el código de producción está mal, no el test (salvo que el test esté mal escrito)
- Si un approach no funciona después de 2 intentos → replantear el diseño
- Si no se entiende por qué algo falla → leer la spec de nuevo antes de tocar código
- Si se detecta deuda técnica mientras se implementa → refactorizar AHORA, no "después"

### Metodología Frontend: Feature-First + Reutilización Explícita

**Feature-first:** Cada HT frontend es una feature vertical completa (UI + hooks + API calls + estado + empty states). No se construyen capas horizontales aisladas.

**Reutilización obligatoria:**
1. **Antes de crear un componente** → verificar si ya existe en `components/shared/` o en el Design System (HT-28)
2. **Si un componente se usa en 2+ features** → extraerlo a `components/shared/` con props genéricas
3. **Patrones compartidos predefinidos** (creados en HT-28):
   - `PageShell` — layout con sidebar + header + breadcrumbs
   - `DataTable` — tabla con sort, filter, paginación cursor-based
   - `FormDialog` — modal con formulario + validación Zod
   - `EmptyState` — estado vacío con ícono + mensaje + CTA
   - `ConfirmDialog` — confirmación destructiva
   - `StatusBadge` — badge de estado con colores semánticos
   - `ScopeIndicator` — indicador de nivel (Global/Tenant/Workspace)
4. **Cada HT documenta** qué componentes reutiliza y cuáles nuevos extrae

---

## Key Architecture Decisions

These are non-negotiable decisions documented in TECH_SPEC v1.4:

1. **No Redis** — PG UNLOGGED table for cache, PL/pgSQL for rate limiting
2. **Adapter assigned per template_type** — not per workspace or template
3. **Resolution chain** via `get_resolution_chain()` PL/pgSQL function
4. **River** for job queue (Go + PG native, no external broker)
5. **OIDC for humans, API Keys for machines** — dual auth
6. **Hexagonal architecture** — ports define contracts, adapters implement
7. **UUIDs v7** — time-ordered, non-sequential
8. **Partitioned tables** — emails and audit_logs by month
9. **Soft delete** — `deleted_at` column, never physical delete
10. **Cursor-based pagination** — no offset, UUIDv7 as cursor

---

## Spec Section Quick Reference

| Section | Content                            |
| ------- | ---------------------------------- |
| §1      | Principles & Glossary              |
| §3      | Complete SQL Schema (16+ tables)   |
| §4      | Partitioning Strategy              |
| §5      | Performance Indices                |
| §6      | App-Level Validations              |
| §7      | Migration Strategy (19 migrations) |
| §8      | Architecture & DI                  |
| §9      | Folder Structure                   |
| §10     | Port Interfaces                    |
| §11     | Domain Models                      |
| §12     | Resolution Engine                  |
| §13     | SendService Flow                   |
| §14     | Middleware Chain                   |
| §15     | API Contract                       |
| §16     | Background Workers                 |
| §17     | Configuration                      |
| §18     | Docker Compose                     |
| §19     | Provider Event Ingestion           |
| §20     | Onboarding Flow                    |
| §21     | Observability                      |
| §22     | DKIM Signing                       |
| §23     | PG Cache                           |
| §24     | Token Bucket Rate Limiting         |
