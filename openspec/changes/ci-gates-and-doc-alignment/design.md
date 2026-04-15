# Design — Alineación de gates CI y documentación

## Objetivo técnico

Hacer que el repositorio tenga una sola verdad operativa para gates y documentación, sin ampliar artificialmente la cobertura ni convertir gates manuales en promesas automáticas.

## Mapa de fuentes de verdad

| Fuente | Rol | Qué debe mandar |
| --- | --- | --- |
| `scripts/run-github-gates.sh` | Contrato ejecutable de gates GitHub/local | Secuencia exacta de comandos |
| `Makefile` | Entrada humana y CI-friendly | Nombres de targets y delegación a scripts |
| `.github/workflows/*.yml` | Orquestación de CI | Qué corre en PR y qué queda manual |
| `web/package.json` | Contrato frontend | Script canónico para tests frontend |
| `README.md` | Onboarding y acceso rápido | Qué ejecutar y con qué alcance |
| `docs/specs/TESTING_STRATEGY.md` | Política de testing | Qué gates existen, qué cubren y qué no |

## Estrategia de simplificación

La simplificación no consiste en inventar más gates; consiste en **reducir la distancia entre intención y realidad**.

### Decisión central

- Mantener el backend PR gate tal como existe: `lint + vet + swagger-check + test`.
- Formalizar un gate frontend real con script de test en `web/package.json`.
- Dejar el system gate como **manual/observacional** si esa sigue siendo su naturaleza.
- Corregir la documentación para que describa lo que realmente se ejecuta.

### Por qué esta estrategia

Porque el problema principal no es la falta de nombres, sino la falta de correspondencia entre:

- lo que CI corre,
- lo que los scripts exponen,
- y lo que la documentación promete.

## Archivos a tocar

### Operación

- `scripts/run-github-gates.sh`
- `Makefile`
- `.github/workflows/backend-gate.yml`
- `.github/workflows/frontend-gate.yml`

### Contrato frontend

- `web/package.json`

### Documentación

- `README.md`
- `docs/specs/TESTING_STRATEGY.md`

## Flujo propuesto

1. Definir taxonomía de gates: backend, frontend y system.
2. Hacer que el frontend tenga un script `test` canónico.
3. Alinear `Makefile`, scripts y workflows con esa taxonomía.
4. Reescribir `README.md` y `TESTING_STRATEGY.md` para que describan gates reales, no aspiracionales.
5. Verificar que no quede ninguna promesa documental sin respaldo operativo.

## Tradeoffs

### Crear nuevos gates vs corregir docs

- **Crear nuevos gates**
  - Ventaja: más cobertura aparente.
  - Riesgo: teatro operativo, mayor mantenimiento y más posibilidades de desalineación.

- **Corregir docs y formalizar solo lo necesario**
  - Ventaja: honestidad, menor complejidad y menor deuda técnica.
  - Riesgo: obliga a aceptar que parte de la cobertura es manual/observacional.

La recomendación es la segunda opción.

### System gate manual vs PR default

- **Mantenerlo manual/observacional**
  - Ventaja: preserva costo y control.
  - Riesgo: no forma parte del gate básico de PR.

- **Moverlo al PR default**
  - Ventaja: más bloqueo automático.
  - Riesgo: eleva demasiado el coste y mezcla validación funcional con observacional.

La recomendación es mantenerlo fuera del PR default si sigue siendo manual.

## Ownership

- `Makefile` y scripts: contrato operativo.
- GitHub workflows: contrato de ejecución en CI.
- `web/package.json`: contrato de validación frontend.
- Documentación: contrato de intención y alcance.

## Resultado esperado

Al terminar este cambio, cualquier lector debe poder responder con precisión:

- qué corre en backend PR,
- qué corre en frontend PR,
- qué queda manual/observacional,
- y dónde está el punto canónico para ejecutar tests frontend.
