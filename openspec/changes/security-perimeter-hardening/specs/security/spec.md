# Delta para security

## ADDED Requirements

### Requirement: SNS inbound must bind to the expected topic and account

El sistema MUST rechazar cualquier mensaje SNS cuyo `TopicArn` no coincida exactamente con el `TopicArn` esperado por configuración. El sistema MUST also validate that the account identifier embedded in the ARN matches the expected account.

#### Scenario: Valid SNS envelope

- GIVEN una notificación SNS firmada correctamente
- AND el `TopicArn` coincide exactamente con el configurado
- AND la cuenta del ARN coincide con la esperada
- WHEN el handler procesa el mensaje
- THEN the message MUST be accepted for further processing

#### Scenario: Wrong topic

- GIVEN una notificación SNS firmada correctamente
- AND el `TopicArn` no coincide con el configurado
- WHEN el handler procesa el mensaje
- THEN the message MUST be rejected
- AND no MUST reach event processing

#### Scenario: Wrong account

- GIVEN una notificación SNS firmada correctamente
- AND el `TopicArn` pertenece a una cuenta distinta de la esperada
- WHEN el handler procesa el mensaje
- THEN the message MUST be rejected

### Requirement: SNS replay protection and deduplication must be explicit

El sistema MUST persist and enforce deduplication for SNS messages using a stable replay key derived from `TopicArn` and `MessageId`. The system MUST reject duplicate deliveries inside the replay window even when the SNS signature is valid.

#### Scenario: Duplicate delivery

- GIVEN una notificación SNS ya aceptada previamente
- AND the same `TopicArn` and `MessageId` are delivered again
- WHEN the handler processes the second delivery
- THEN the second delivery MUST be rejected as a replay or duplicate
- AND the underlying event MUST NOT be processed twice

#### Scenario: Stale replay

- GIVEN una notificación SNS fuera de la ventana de replay configurada
- WHEN the handler evaluates it
- THEN the message MUST be rejected

#### Scenario: Unique message

- GIVEN una notificación SNS nueva y válida
- WHEN the handler processes it for the first time
- THEN the message MUST be accepted
- AND the replay key MUST be recorded durably

### Requirement: Outbound webhook delivery must not follow redirects automatically

The webhook delivery worker MUST fail on any HTTP redirect response and MUST NOT follow redirects automatically.

#### Scenario: Direct success

- GIVEN un endpoint webhook que responde 200 directamente
- WHEN el worker entrega el payload
- THEN the delivery MUST succeed

#### Scenario: Redirect response

- GIVEN un endpoint webhook que responde 302 or 307
- WHEN el worker entrega el payload
- THEN the delivery MUST fail
- AND the worker MUST NOT issue a second request to the redirect target

### Requirement: Public media fetch must deny by default and pin the resolved destination

El sistema MUST require an explicit allowlist for public media hosts. The system MUST resolve the host, validate that every resolved address is public-safe, and pin the chosen destination for the lifetime of the request. Any redirect hop MUST be revalidated under the same policy, and any rebinding to a disallowed destination MUST be rejected.

#### Scenario: Allowed public host

- GIVEN una URL de media cuyo host está permitido
- AND the resolved address is public-safe
- WHEN the fetch starts
- THEN the request MUST proceed using a pinned destination

#### Scenario: Private destination

- GIVEN una URL de media que resuelve a una dirección privada, loopback, link-local, reserved o similar
- WHEN the system validates the URL
- THEN the request MUST be rejected

#### Scenario: DNS rebinding attempt

- GIVEN una URL de media permitida inicialmente
- AND the host changes resolution before or during the request to a disallowed address
- WHEN the request is retried or redirected
- THEN the system MUST reject the request

### Requirement: External integrations must not accept bearer token via query string

El sistema MUST accept the external integration bearer token only through the dedicated header transport. The system MUST reject requests that provide the token only as a query string parameter.

#### Scenario: Header token

- GIVEN una request de integración externa con el header dedicado presente
- WHEN the middleware authenticates the request
- THEN the request MUST be evaluated using the header token

#### Scenario: Query token only

- GIVEN una request de integración externa con `?token=` presente
- AND no header token is provided
- WHEN the middleware authenticates the request
- THEN the request MUST be rejected

### Requirement: Negative security coverage must exist for all perimeter rules

El sistema MUST include automated tests that cover the negative paths for SNS topic/account mismatches, replay/duplicate delivery, webhook redirects, media rebinding/private destinations, and external integration query tokens.

#### Scenario: Security regression suite

- GIVEN los tests del perímetro de seguridad
- WHEN the suite runs
- THEN each perimeter rule MUST have at least one negative scenario and one positive scenario
