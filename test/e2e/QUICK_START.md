# E2E Quick Start

## Clonaste el repo y querés correr E2E

### Gate determinística completa

```bash
make test-e2e
```

### Solo SES

```bash
make test-e2e-ses
```

## Qué hace

No necesitás levantar nada a mano.  
El harness E2E crea con **Testcontainers** todo lo necesario:

- PostgreSQL
- Mailpit
- backend Senda
- MiniStack
- aws-sim bridge

## Prerrequisitos

- Docker corriendo
- Go 1.25+

## Suite SES incluida

`make test-e2e-ses` cubre:

- provisionamiento SES completo
- setup de identities
- envío real por SES simulado
- delivery / bounce / complaint
- deprovision al eliminar adapter
- replay SNS firmado

## Si querés correr el comando raw

```bash
go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run 'TestSESLifecycle0[1-4]_|TestSNSReplay01_'
```
