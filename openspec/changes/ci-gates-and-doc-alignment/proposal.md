# Alineación de gates CI y documentación

## Problema

Hoy existe una diferencia clara entre lo que el repositorio **dice** que valida y lo que realmente valida la CI:

- `README.md` y `docs/specs/TESTING_STRATEGY.md` prometen una cobertura y unos gates más amplios de lo que la CI ejecuta de forma real.
- El backend PR gate sí está definido y es consistente: `lint + vet + swagger-check + test`.
- `web/package.json` no expone un script `test`, aunque el frontend ya contiene tests.
- El gate de system puede seguir fuera del PR default si es manual/observacional, pero hoy esa distinción no está documentada con honestidad suficiente.

El problema no es solo de documentación: cuando la verdad operativa y la verdad documental divergen, se crea una falsa expectativa de calidad y se vuelve más difícil razonar sobre qué rompe realmente una PR.

## Solución propuesta

Voy a definir una única verdad operativa entre:

- `Makefile`
- workflows de GitHub
- `README.md`
- `docs/specs/TESTING_STRATEGY.md`
- scripts frontend

La solución debe:

1. formalizar la ejecución de tests frontend dentro del flujo normal;
2. mantener el backend PR gate como la secuencia real ya existente;
3. documentar el system gate con honestidad si permanece manual/observacional;
4. alinear nombres, alcance y semántica entre docs y ejecución real;
5. evitar inventar gates nuevos solo para sostener una narrativa.

## Alcance

### En alcance

- `README.md`
- `docs/specs/TESTING_STRATEGY.md`
- `Makefile`
- `.github/workflows/*` relacionados con gates
- `scripts/run-github-gates.sh`
- `web/package.json`

### Fuera de alcance

- Cambios de comportamiento de producto.
- Ampliar el system gate para convertirlo en un PR blocker por defecto.
- Reescrituras grandes del runner system o del stack E2E.
- Añadir cobertura ficticia o “de papel”.

## Alternativas consideradas

1. **Crear nuevos gates para que la documentación “suene” correcta.**  
   Rechazado: añade complejidad sin verdad operativa real y vuelve a mover el problema.

2. **Solo corregir documentación.**  
   Insuficiente: si existe un test frontend real, el flujo normal debe poder ejecutarlo.

3. **Agregar un script `test` al frontend y alinear docs/workflows/Makefile con los gates reales.**  
   Elegido: es la opción mínima que corrige la desalineación sin sobrediseñar el sistema.

## Rollback

Si alguna parte de la alineación introduce fricción operativa, el rollback debe ser directo:

- revertir los cambios de scripts/workflows relacionados;
- restaurar la redacción previa de documentación;
- mantener intacto el backend PR gate real;
- no tocar el comportamiento de producto.

## Reviewer final

James
