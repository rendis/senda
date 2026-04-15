# Proposal: composition-boundary-slimming

## Intent

La re-auditoría ciega volvió a señalar tres problemas de frontera que ya no deberían convivir en el mismo nivel: `internal/app/app.go` sigue actuando como composition root demasiado cargado, `internal/http/server.go` sigue funcionando como switchboard grande, y `internal/http/handler/provider_webhook.go` sigue con conocimiento SES que pertenece a un boundary provider-specific.

Este cambio busca adelgazar de verdad la frontera de composición sin alterar comportamiento visible: la raíz de la app debe orquestar, el router debe registrar por superficie, y la lógica SES debe salir del handler HTTP.

## Scope

### In Scope
- Reducir `app.Bootstrap` a wiring compartido y delegación explícita.
- Particionar el registro de rutas por superficie con registradores dedicados.
- Extraer la lógica SES/SNS específica fuera del handler HTTP hacia un boundary provider-specific.
- Añadir validación representativa y E2E/autónoma para demostrar que los boundaries quedaron más limpios sin regresión.

### Out of Scope
- Cambios de negocio, contratos públicos o rutas nuevas no requeridas por esta limpieza.
- Migraciones de base de datos.
- Reescritura de providers, workers o del modelo de dominio.
- Cambios fuera de `senda/`.

## Architecture Direction

- El composition root debe dejar de ser el lugar donde se mezclan decisiones de infraestructura, construcción de servicios y registro de rutas.
- El servidor HTTP debe dejar de concentrar todas las superficies en un solo bloque secuencial; cada superficie debe tener un registrador propio y legible.
- La lógica específica de SES debe vivir detrás de un paquete provider-specific para que el handler HTTP quede como adaptador de transporte.

## Alternatives Considered

### Opción A: solo mover helpers dentro de los mismos archivos
**Pros**: menor churn, más rápido de implementar.

**Contras**: no crea fronteras reales; sigue habiendo archivos demasiado cargados y el switchboard sigue siendo difícil de razonar.

**Veredicto**: rechazada.

### Opción B: registradores por superficie + boundary SES específico
**Pros**: reduce el tamaño cognitivo de cada archivo, hace explícita la ownership por superficie y separa transporte de lógica provider-specific.

**Contras**: introduce más archivos y un poco más de wiring.

**Veredicto**: recomendada.

### Opción C: rediseño completo del sistema de composición
**Pros**: máxima limpieza conceptual.

**Contras**: demasiado churn para el problema actual; arrastra riesgo innecesario y difiere la entrega.

**Veredicto**: rechazada.

## Dependencies and Parallelism

- **Dependencia lógica**: este cambio asume como base el trabajo ya cerrado de `surface-modularization-and-sdk-hardening`, porque ese stream ya dejó explícitas las superficies y los seams del SDK.
- **Paralelismo seguro**: puede avanzar en paralelo con `send-path-amortization-and-cache-precision` siempre que ese stream no vuelva a tocar `internal/app/app.go` ni `internal/http/server.go`.
- **No bloquea ni depende** de `autonomous-e2e-isolation`, `security-perimeter-hardening` o `ci-gates-and-doc-alignment`; esos streams ya resolvieron la infraestructura de validación y no necesitan esta limpieza para existir.
- **Reviewer final esperado**: Volta.

## Risks

- Partir el router en registradores pequeños pero seguir escondiendo ownership dentro de funciones utilitarias gigantes.
- Extraer SES fuera del handler HTTP pero dejar la semántica de parsing todavía acoplada al transporte.
- Cambiar la estructura sin preservar la cobertura de rutas existentes.

## Rollback Plan

El rollback es de código solamente: revertir los registradores nuevos, restaurar el wiring inline y regresar la lógica SES al handler si fuera necesario. No hay cambios de esquema ni contratos que obliguen a una migración inversa.

## Success Criteria

- `internal/app/app.go` queda como orquestador mínimo, no como lugar donde vive toda la composición.
- `internal/http/server.go` deja de ser un switchboard monolítico y pasa a delegar en registradores por superficie.
- La lógica SES específica deja de vivir dentro del handler HTTP y pasa a un boundary provider-specific.
- La validación representativa y/o E2E autónoma demuestra que las rutas y los flujos existentes siguen funcionando.
