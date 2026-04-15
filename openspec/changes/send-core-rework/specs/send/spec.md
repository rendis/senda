# Spec — Send Core Rework

## 1. Core de envío con límites explícitos

### 1.1 Requisitos

1.1.1 El core de envío **MUST** estar dividido en use cases explícitos con responsabilidades separadas.  
1.1.2 Ningún use case **MUST** depender directamente de la capa HTTP.  
1.1.3 La orquestación del envío **MUST** poder testearse sin pasar por el handler.  
1.1.4 La resolución de contexto, la evaluación de supresión, la persistencia y el enqueue **MUST** ser verificables como unidades independientes.  
1.1.5 La implementación **MUST NOT** depender de una única función monolítica para toda la ruta de envío.

### 1.2 Escenarios

#### Escenario: envío aceptado con contexto resuelto

**Given** un request válido con un `ref`, destinatarios y variables  
**When** el core resuelve tenant, workspace, template, adapter y sender identity  
**Then** el flujo **MUST** producir un contexto de envío completo antes de persistir nada  
**And** el resultado final **MUST** mantener el tracking por destinatario.

#### Escenario: use cases independientes

**Given** el core separado en use cases  
**When** se prueba la resolución de contexto en aislamiento  
**Then** la prueba **MUST** poder validar el resultado sin mockear el worker  
**And** la prueba de persistencia **MUST** poder ejecutarse sin resolver templates otra vez.

## 2. Supresión batched

### 2.1 Requisitos

2.1.1 La evaluación de supresión **MUST** operar sobre un batch de destinatarios normalizados.  
2.1.2 El core **MUST NOT** hacer una consulta de supresión por cada destinatario de forma secuencial.  
2.1.3 La consulta batched **MUST** preservar el motivo de supresión por destinatario cuando exista.  
2.1.4 La consulta batched **MUST** combinar supresión global y workspace en el mismo contrato de salida.  
2.1.5 Los destinatarios no suprimidos **MUST** continuar por el pipeline sin bloquear a los suprimidos.  
2.1.6 Los destinatarios suprimidos **MUST** quedar registrados con su estado y razón correspondiente.

### 2.2 Escenarios

#### Escenario: batch mixto de destinatarios

**Given** un batch con destinatarios aceptados, suprimidos globalmente y suprimidos a nivel workspace  
**When** el core evalúa la supresión  
**Then** el resultado **MUST** separar los tres casos por destinatario  
**And** los destinatarios aceptados **MUST** seguir hacia persistencia y enqueue  
**And** los destinatarios suprimidos **MUST** quedar persistidos con el motivo correcto.

#### Escenario: eliminación del chequeo secuencial

**Given** un envío con múltiples destinatarios  
**When** el core procesa la supresión  
**Then** el número de consultas al store de supresión **MUST** ser proporcional al batch, no al destinatario individual  
**And** la ruta no **MUST** depender de un loop de consulta por cada email.

## 3. Rate limiting con menor contención

### 3.1 Requisitos

3.1.1 El rate limiting por adapter **MUST** reducir la contención frente al camino actual con una adquisición por mensaje bajo `FOR UPDATE`.  
3.1.2 El contrato de rate limit **SHOULD** permitir reservas o bursts pequeños por adapter para amortiguar la presión de 30 workers.  
3.1.3 La implementación **MUST** seguir respetando el límite configurado por adapter.  
3.1.4 La solución **MUST** ser segura bajo concurrencia y no permitir sobreenvío por encima del límite.  
3.1.5 La ruta de rate limit **MUST** seguir siendo observable y testeable.

### 3.2 Escenarios

#### Escenario: 30 workers compiten por el mismo adapter

**Given** 30 workers procesando jobs del mismo adapter  
**When** el core solicita tokens en paralelo  
**Then** el sistema **MUST** mostrar menos serialización que el camino actual por mensaje  
**And** el límite por segundo **MUST** seguir cumpliéndose  
**And** el benchmark **MUST** registrar la mejora o, si no existe, documentarla con evidencia.

#### Escenario: reserva de tokens por burst

**Given** un adapter con límite configurado  
**When** el core solicita una reserva pequeña de tokens  
**Then** la reserva **MUST** comportarse como una sección crítica única  
**And** el resto del pipeline **MUST** poder reutilizar esa reserva sin recalcular la contención por cada mensaje.

## 4. Skinny rows y payload frío

### 4.1 Requisitos

4.1.1 La fila operativa de email **MUST** contener solo los campos calientes necesarios para listar, rastrear y ejecutar el estado.  
4.1.2 Los snapshots amplios y el payload frío **MUST** moverse a un almacenamiento separado.  
4.1.3 El worker **MUST** cargar el payload frío solo cuando realmente lo necesite.  
4.1.4 Las consultas de listado y detalle **MUST** evitar cargar el payload frío por defecto cuando no sea necesario.  
4.1.5 El split hot/cold **MUST** preservar la trazabilidad por `tracking_id` y `provider_message_id`.

### 4.2 Escenarios

#### Escenario: persistencia caliente mínima

**Given** un envío aceptado  
**When** el core persiste el mensaje  
**Then** la fila operativa **MUST** guardar identidad, estado, tracking y retry state  
**And** los snapshots amplios **MUST** quedar fuera de la fila caliente.

#### Escenario: worker con payload diferido

**Given** un job dequeued por River  
**When** el worker carga el email para enviar  
**Then** primero **MUST** leer la fila caliente  
**And** solo después **MUST** leer el payload frío necesario para renderizar y enviar.

## 5. Validación E2E y bench funcional

### 5.1 Requisitos

5.1.1 El cambio **MUST** incluir tests unitarios de los nuevos use cases.  
5.1.2 El cambio **MUST** incluir tests de integración para el batch de supresión, el split hot/cold y el nuevo rate limiting.  
5.1.3 El cambio **MUST** incluir una validación E2E autónoma del flujo de envío reescrito.  
5.1.4 El cambio **MUST** incluir un benchmark funcional del hot path con comparación contra el baseline.  
5.1.5 El benchmark **MUST** reportar round-trips, contención de lock y costo de supresión en batch.  
5.1.6 El benchmark **MAY** justificar trabajo adicional en cache o dashboard únicamente si demuestra impacto real en la ruta caliente.

### 5.2 Escenarios

#### Escenario: validación autónoma del cambio

**Given** el core reescrito y el stack de pruebas aislado  
**When** se ejecuta la batería E2E autónoma  
**Then** el flujo de envío **MUST** pasar sin depender de recursos compartidos ambiguos  
**And** el resultado del benchmark **MUST** quedar registrado para revisión técnica.

#### Escenario: decisión guiada por benchmark

**Given** un benchmark funcional del hot path  
**When** el benchmark muestra que cache o dashboard no afectan la ruta caliente  
**Then** esas áreas **MUST NOT** entrar en el alcance de este cambio  
**And** si sí hay impacto medible, la extensión del alcance **MAY** justificarse con evidencia.
