# Tasks — Alineación de gates CI y documentación

## 1. Taxonomía de gates

1.1 Confirmar la taxonomía operativa real: backend PR, frontend PR, system manual/observacional y system nightly.  
1.2 Definir la nomenclatura única que usarán `Makefile`, scripts, workflows y documentación.  
1.3 Asegurar que no se mezclen gates de validación automática con gates observacionales.

## 2. Contrato frontend

2.1 Introducir el script `test` canónico en `web/package.json`.  
2.2 Definir el alcance exacto del script para que represente el flujo normal de tests frontend.  
2.3 Alinear el gate frontend para que invoque ese script como entrada principal.

## 3. Alineación operativa

3.1 Revisar `scripts/run-github-gates.sh` para que backend y frontend reflejen la taxonomía aprobada.  
3.2 Revisar `Makefile` para que los targets expuestos no prometan más de lo que realmente ejecutan.  
3.3 Verificar que los workflows de GitHub usen los mismos nombres y secuencias que los scripts locales.

## 4. Honestidad documental

4.1 Reescribir `README.md` para describir gates reales y no aspiracionales.  
4.2 Reescribir `docs/specs/TESTING_STRATEGY.md` para separar claramente lo implementado de lo deseable.  
4.3 Documentar explícitamente que el system gate puede permanecer manual/observacional si no forma parte del PR default.

## 5. Verificación de consistencia

5.1 Comparar docs, Makefile, scripts y workflows para detectar divergencias de nombres, alcance o secuencia.  
5.2 Confirmar que el frontend tiene un entrypoint único de test.  
5.3 Confirmar que no se introducen nuevos gates “de papel” sin respaldo operativo.

## 6. Cierre del ciclo

6.1 Revisar el estado final después de que `autonomous-e2e-isolation` esté cerrado, si esa dependencia afecta el wording final del system gate.  
6.2 Consolidar el resultado final del cambio.  
6.3 Dejar listo el cambio para revisión por `James`.
