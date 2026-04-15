# Tasks — Autonomous E2E Isolation

## 1. Fundación de aislamiento

1.1 Definir el contrato común de scope para spec/worktree/run.  
1.2 Implementar derivación única de nombres para red, contenedores y artefactos.  
1.3 Alinear el scope entre harness E2E, stack de test y orquestación system.

## 2. Implementación funcional

2.1 Parametrizar `test/e2e/harness_test.go` para eliminar nombres globales fijos.  
2.2 Parametrizar `internal/teststack/stack.go` para dejar de usar prefijos fijos.  
2.3 Actualizar `test/system/system-runner.sh` para que los reportes y logs vivan bajo un root por scope.  
2.4 Mantener cleanup garantizado con el mismo scope usado en `up`.

## 3. Contrato de artefactos

3.1 Emitir env report y stage results con identidad de scope explícita.  
3.2 Mantener stage logs, junit y run-result con paths consistentes.  
3.3 Asegurar que un fallo en cualquier etapa no elimine la evidencia generada.

## 4. Validación

4.1 Agregar smoke de dos stacks simultáneos sin colisión.  
4.2 Verificar que el camino determinístico excluye heavy stages por defecto.  
4.3 Validar que visual/a11y/chaos solo corren bajo activación explícita.  
4.4 Revisar cleanup final y consistencia de artefactos.

## 5. Verify-report

5.1 Consolidar resultados de la validación final.  
5.2 Registrar riesgos residuales o dependencias externas.  
5.3 Dejar listo el cambio para el siguiente paso del ciclo SDD.
