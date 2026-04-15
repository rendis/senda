# Spec — Gates CI y alineación documental

## 1. Contexto

El repositorio debe exponer una verdad operativa única para sus gates de validación. Esa verdad debe ser coherente entre scripts, Makefile, workflows, frontend y documentación.

## 2. Requisitos

### 2.1 Taxonomía de gates

- El backend PR gate **MUST** corresponder a la secuencia real: `lint`, `vet`, `swagger-check` y `test`.
- El frontend PR gate **MUST** tener una entrada canónica para ejecutar tests frontend en el flujo normal.
- El system gate **MAY** permanecer fuera del PR default si sigue siendo manual/observacional.
- Cualquier gate manual/observacional **MUST** quedar etiquetado explícitamente como tal en la documentación.

### 2.2 Honestidad documental

- `README.md` **MUST** describir solo gates y alcances que existan realmente.
- `docs/specs/TESTING_STRATEGY.md` **MUST** distinguir entre:
  - validación automática,
  - validación manual/observacional,
  - y aspiraciones que todavía no forman parte del flujo normal.
- Ningún documento **MUST** sugerir que CI ejecuta un gate que en realidad no existe.
- Si una parte del sistema se mantiene manual por coste o naturaleza operativa, la documentación **MUST** decirlo de forma explícita.

### 2.3 Script canónico de tests frontend

- `web/package.json` **MUST** exponer un script `test`.
- Ese script **MUST** representar el entrypoint canónico para los tests frontend dentro del flujo normal.
- El gate frontend **MUST** usar ese entrypoint en lugar de depender de comandos ad hoc dispersos.

### 2.4 Consistencia entre CI, scripts y docs

- `Makefile`, `scripts/run-github-gates.sh`, `.github/workflows/*.yml`, `README.md` y `docs/specs/TESTING_STRATEGY.md` **MUST** usar la misma taxonomía de gates.
- Los nombres de los gates **MUST** significar la misma cosa en todos los puntos de entrada.
- Si un workflow se mantiene como `workflow_dispatch`, la documentación **MUST** reflejar que su ejecución es manual.
- El sistema **SHOULD** evitar introducir un nuevo gate si el objetivo real es solo corregir la descripción de uno existente.

## 3. Escenarios

### Escenario 1 — Backend PR gate consistente

**Given** que una PR toca código backend  
**When** se ejecuta el gate de backend  
**Then** el pipeline debe ejecutar `lint`, `vet`, `swagger-check` y `test` en esa secuencia lógica  
**And** la documentación debe describir exactamente ese alcance, sin añadir cobertura inexistente.

### Escenario 2 — Tests frontend en el flujo normal

**Given** que el frontend contiene tests  
**When** un desarrollador ejecuta el gate frontend normal  
**Then** debe existir un script `test` en `web/package.json`  
**And** ese script debe ser el punto canónico para invocar los tests frontend  
**And** el gate frontend debe usar ese script como entrada principal.

### Escenario 3 — System gate manual/observacional

**Given** que el system gate se conserva fuera del PR default  
**When** alguien lea `README.md` o `TESTING_STRATEGY.md`  
**Then** debe quedar claro que ese gate es manual/observacional  
**And** no debe presentarse como bloqueo automático del PR.

### Escenario 4 — Documentación honesta

**Given** que la documentación describe la estrategia de testing  
**When** el lector compare docs con `Makefile` y workflows  
**Then** no debe encontrar promesas de cobertura que CI no ejecuta  
**And** cualquier limitación operativa debe quedar explícita.

### Escenario 5 — Taxonomía única

**Given** que un mismo nombre de gate aparece en varias partes del repositorio  
**When** se comparen scripts, workflows y docs  
**Then** ese nombre debe significar el mismo alcance en todos lados  
**And** no debe existir una semántica duplicada o contradictoria.
