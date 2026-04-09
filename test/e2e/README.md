# Senda E2E Test Suite

Este directorio contiene los tests E2E del backend de Senda.  
**No requieren levantar un stack manualmente**: el harness usa **Testcontainers** y se autogestiona solo.

## Qué necesita alguien que clone el repo

### Prerrequisitos

- Docker Desktop / Docker Engine corriendo
- Go 1.25+
- `git clone` del repo

### Comando recomendado

```bash
make test-e2e
```

Ese target ejecuta la **gate determinística** completa, incluyendo:

- core gate
- CRUD flows
- error flows
- happy paths
- **injector runtime precedence (`reqBody > code > default`, con locked fields)**
- **SES lifecycle sobre aws-sim + MiniStack**
- **SNS signed replay**

## Ejecutar solo la suite SES

```bash
make test-e2e-ses
```

Eso corre:

- `TestSESLifecycle01_HappyPath`
- `TestSESLifecycle02_BounceCreatesSuppression`
- `TestSESLifecycle03_ComplaintCreatesWorkspaceSuppression`
- `TestSESLifecycle04_DeleteAdapterDeprovisionsAWSSim`
- `TestSNSReplay01_SignedNotificationsRemainSupported`

## Qué cubre la suite SES

### 1. Provisionamiento completo

- creación del adapter SES vía API
- auto-provision tracking vía endpoint público
- configuration set
- SNS topic
- event destination
- subscription
- persistencia de `configuration_set_name`
- pasos de provisioning en DB

### 2. Acondicionamiento para envío real

- sync de identity de dominio desde el provider simulado
- creación de identity manual de sender
- set default identity

### 3. Envío real

- `/api/v1/send`
- persistencia de `provider_message_id`
- transición a `sent`

### 4. Recepción de eventos SES/SNS

- `Delivery`
- `Bounce`
- `Complaint`
- actualización de estado
- suppressions

### 5. Eliminación / deprovision

- delete del adapter
- unsubscribe
- delete event destination
- delete SNS topic
- delete configuration set

### 6. Firma SNS

- replay separado con envelope SNS firmado
- validación de la ruta criptográfica del webhook

## Arquitectura del harness SES

La suite SES no depende de AWS real ni de LocalStack.

Usa:

- **MiniStack** como backend AWS simulado por defecto
- **aws-sim bridge** test-only delante de MiniStack

El bridge:

- proxyfea lo soportado por MiniStack
- emula APIs SES faltantes
- mantiene estado de tracking
- dispara eventos SES determinísticos

### Importante

MiniStack en este entorno no hace siempre el fanout HTTP SNS de forma suficientemente confiable para una suite determinística.  
Por eso el **bridge aws-sim** entrega el envelope SNS al webhook de Senda cuando hace falta.  

Eso significa que el pipeline cubierto sigue siendo real desde el punto de vista de Senda:

`SES notification -> SNS envelope -> webhook -> parser -> EventProcessor -> DB/suppressions`

## Suite de injectors runtime

El archivo:

- `/Users/rendis/Documents/Projects/Libraries/senda/test/e2e/injector_runtime_test.go`

cierra la cobertura E2E del nuevo modelo de injectors workspace-only con `default_value` +
`allow_overwrite`.

Casos cubiertos:

1. default-only
2. `reqBody.injectors` gana sobre code/default
3. field locked ignora reqBody/code y usa default
4. code gana sobre default cuando no hay reqBody
5. reqBody gana sobre code
6. fallback parcial por field dentro del mismo injector
7. `null` explícito en reqBody
8. string vacío explícito en reqBody
9. management template test-send respeta la misma precedencia
10. management template bulk-send propaga `injectors` por item
11. sanity check con SES/MiniStack para confirmar que el render nuevo sobrevive un send real

La suite usa un binario E2E test-only:

- `/Users/rendis/Documents/Projects/Libraries/senda/cmd/senda-e2e/main.go`

Ese binario registra un code injector determinístico `student` cuando
`SENDA_E2E_ENABLE_CODE_INJECTORS=true`.

### Ejecutar solo injectors runtime

```bash
go test -tags=e2e -v -count=1 -timeout 20m ./test/e2e/ -run 'TestInjectorRuntime0[1-3]_'
```

### Qué valida exactamente la precedencia

- `allow_overwrite=false` → siempre default
- `allow_overwrite=true` → `reqBody.injectors > code > default_value`
- la precedencia es **por field**, no por injector completo
- `event.*` sigue separado de `injector.*`

## Variables útiles

Normalmente **no hace falta setear nada**.

Opcionales:

- `SENDA_AWS_SIM_IMAGE` — override de imagen MiniStack
- `SENDA_BASE_URL` — base URL del backend para harness externo
- `MAILPIT_URL` — base URL de Mailpit para harness externo
- `SENDA_E2E_JWT_SECRET` — secret del modo test JWT

## Comandos directos

### Gate completa

```bash
go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run '^(TestCore|TestCRUD|TestE|TestF|TestS)'
```

### Solo SES

```bash
go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run 'TestSESLifecycle0[1-4]_|TestSNSReplay01_'
```

## Archivos importantes

- `/Users/rendis/Documents/Projects/Libraries/senda/test/e2e/ses_lifecycle_test.go`
- `/Users/rendis/Documents/Projects/Libraries/senda/test/e2e/sns_replay_test.go`
- `/Users/rendis/Documents/Projects/Libraries/senda/test/e2e/sns_replay_harness_test.go`
- `/Users/rendis/Documents/Projects/Libraries/senda/test/e2e/aws_sim_helpers_test.go`
- `/Users/rendis/Documents/Projects/Libraries/senda/internal/teststack/awssim/bridge.go`

## Regla práctica

Si querés validar que un clon fresco corre la cobertura SES, el comando correcto es:

```bash
make test-e2e-ses
```
