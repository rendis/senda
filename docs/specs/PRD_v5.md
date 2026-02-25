# PRD: Senda — Plataforma de Orquestación de Email

**Versión:** 5.0 (Draft para revisión)

**Fecha:** 2026-02-16

**Autor:** Rey + Claude (iteración colaborativa)

**Estado:** En revisión — nada en este documento es versión final

---

## 1. Problem Statement

Las empresas que gestionan múltiples marcas, regiones o clientes necesitan enviar emails transaccionales y de notificación desde diferentes contextos, cada uno con su propia identidad visual, datos de dominio y proveedores de envío. Hoy resuelven esto de tres formas, todas deficientes:

Conectar cada aplicación directamente a un proveedor (SES, Gmail), dispersando la lógica de envío, duplicando templates, e imposibilitando la visibilidad centralizada. Usar un SaaS como SendGrid o Resend, cediendo control sobre la infraestructura y aceptando limitaciones de customización. Usar herramientas open source como Listmonk o Postal, que no ofrecen multi-tenancy jerárquico con herencia de configuración, ni trazabilidad por ID de negocio externo.

**¿Quién experimenta este problema?** Empresas con múltiples productos, marcas, regiones o clientes que necesitan enviar emails transaccionales de forma centralizada.

**¿Costo de no resolverlo?** Templates duplicados, emails que fallan sin visibilidad, imposibilidad de auditar la comunicación por email, dependencia de SaaS con costos crecientes, y deuda técnica por integraciones directas.

---

## 2. Goals

**User Goals:**

1. Un punto centralizado donde cualquier aplicación envía emails, con visibilidad completa del ciclo de vida.
2. Gestión jerárquica en 3 niveles (Global → Tenant → Workspace) con herencia automática y override donde sea necesario.
3. Trazabilidad de todos los emails de un caso de negocio vía external_id, cross-workspace y cross-tenant.
4. Templates reutilizables con editor visual (drag-and-drop + inline) para usuarios no técnicos.
5. Inyectores de datos tipados que proveen contexto automáticamente según la jerarquía.
6. Addressing determinístico: `tenantCode:workspaceCode:templateType` resuelve siempre al template publicado, sin manejar IDs internos.
7. Soporte de i18n en templates con asistencia de IA para traducciones y armado de contenido.

**Business Goals:**

1. Reducir N integraciones directas a una sola plataforma centralizada.
2. Auditar el 100% de la comunicación por email en un solo lugar.
3. Proyecto open source sin dependencia de SaaS.

---

## 3. Non-Goals

1. **Senda NO es un proveedor de envío de email.** No compite con SES o Gmail. Es una capa de orquestación que usa esos proveedores como transporte vía adapters.

2. **Senda NO controla qué evento dispara un email.** La lógica de "cuándo enviar" vive fuera de Senda. Un servicio externo decide qué enviar y llama a la API con los datos.

3. **Senda NO es una herramienta de email marketing.** No gestiona listas de suscriptores, segmentación, A/B testing de campañas, ni scheduling de marketing.

4. **Senda NO es un Identity Provider.** La autenticación del dashboard se delega a OIDC externo. Senda gestiona membresía y roles (autorización), pero no autenticación (no hay signup, password recovery, ni MFA propios).

5. **Senda NO expone un SMTP relay.** Comunicación exclusivamente vía REST API.

---

## 4. Conceptos Fundamentales

### 4.1. Jerarquía: Global → Tenant → Workspace

Senda opera con **tres niveles jerárquicos**. Los nombres son genéricos — cada empresa define qué significa cada nivel.

**Global** es la capa raíz. Administrada por superadmins. Define defaults heredados por toda la plataforma: templates base, inyectores corporativos, adapters de envío por defecto, dominios verificados, configuraciones de plataforma.

**Tenant** es el primer nivel de agrupación (país, región, división, línea de negocio). Cada instalación tiene al menos 1 tenant. Cada tenant se identifica por un **código único** (slug): `latam`, `europe`, `division-norte`.

**Workspace** es la unidad operativa granular (cliente, marca, producto, equipo). Cada tenant tiene al menos 1 workspace. Cada workspace se identifica por un **código único dentro de su tenant**: `acme-corp`, `brand-x`, `soporte`.

#### Workspace de Sistema (`_system`)

Cada tenant tiene un workspace especial auto-creado llamado `_system`. Este workspace:

- Es donde se configuran templates, inyectores, adapters y dominios que **heredan todos los workspaces del tenant**.
- No se puede eliminar ni renombrar.
- No se pueden enviar emails desde él directamente.
- Es el equivalente a "configuración a nivel tenant" pero gestionado de forma uniforme como un workspace.

#### Cadena de Resolución

Cuando el sistema necesita resolver cualquier recurso (template, inyector, adapter, dominio, configuración), busca en esta cadena:

```
Workspace → Tenant _system → Global
```

La primera coincidencia gana. Esto permite definir defaults arriba y overrides abajo.

**Error si no se resuelve:** Si un recurso necesario no se encuentra en ningún nivel de la cadena (ej: no hay adapter configurado), Senda retorna `422 Unprocessable Entity` con un mensaje descriptivo: `"No email adapter configured for workspace 'acme' in tenant 'latam'. Configure an adapter at workspace, tenant (_system), or global level."` El workspace queda inutilizable para envío hasta que se configure el recurso faltante.

**Ejemplo concreto:**

```
Global
├── Inyector "brand": {logo: corp-logo.png, name: "MiEmpresa", email: soporte@miempresa.com}
├── Template "welcome" (global)
├── Adapter: SES us-east-1
├── Dominio verificado: miempresa.com
│
├── Tenant "latam" (código: latam)
│   ├── _system workspace
│   │   ├── Inyector "brand": {logo: latam-logo.png}  ← override parcial, hereda name y email
│   │   ├── Template "welcome" (LATAM)                 ← override del global
│   │   ├── Adapter: SES sa-east-1                     ← override del global
│   │   └── Dominio verificado: latam.miempresa.com    ← adicional al global
│   │
│   ├── Workspace "acme" (código: acme)
│   │   ├── Inyector "brand": {name: "Acme Corp"}      ← override parcial
│   │   ├── Template "welcome" (Acme)                   ← override del tenant
│   │   └── (hereda adapter de _system: SES sa-east-1)
│   │
│   └── Workspace "beta" (código: beta)
│       └── (hereda todo de _system y global)
```

En este ejemplo, un email "welcome" enviado desde `latam:acme` usa:
- Template "welcome" de Acme (workspace).
- Inyector "brand" mergeado: `{logo: latam-logo.png, name: "Acme Corp", email: soporte@miempresa.com}`.
- Adapter SES sa-east-1 (de `_system`).
- Dominio: latam.miempresa.com (de `_system`) o miempresa.com (de global), según configuración del from address.

Un email "welcome" desde `latam:beta` usa:
- Template "welcome" de LATAM (_system).
- Inyector "brand": `{logo: latam-logo.png, name: "MiEmpresa", email: soporte@miempresa.com}`.
- Adapter SES sa-east-1 (de `_system`).

#### Aislamiento Cross-Tenant

Los roles no-superadmin están **estrictamente aislados** por tenant:
- Un `tenant-admin` de "latam" NO tiene visibilidad alguna sobre el tenant "europe" ni sus workspaces.
- Un `workspace-admin` de "latam:acme" NO puede ver ni acceder a "latam:beta".
- Solo los `superadmin` tienen visibilidad cross-tenant.
- Las API Keys están scoped a un workspace y no pueden operar fuera de él.

### 4.2. Códigos (Slugs)

Tenants y workspaces se identifican por **códigos** en formato slug:

- Lowercase alfanumérico + guiones: `[a-z][a-z0-9-]*`
- Mínimo 2, máximo 50 caracteres.
- Debe empezar con letra.
- Sin guiones consecutivos (`--`).
- Sin guion al final.
- Reservados: `_system`, `global`, `admin`, `api`, `system` (no pueden usarse como códigos de workspace o tenant).

**Unicidad:**
- Código de tenant: **único global** (no pueden existir dos tenants con el mismo código).
- Código de workspace: **único dentro de su tenant** (dos tenants pueden tener un workspace "main", pero un mismo tenant no puede tener dos "main").

**Inmutabilidad:** Los códigos son inmutables después de la creación. Cambiar un código rompería integraciones externas que usan el addressing `tenantCode:workspaceCode:templateType`.

### 4.3. Inyectores (Data Injectors)

Un **inyector** es un conjunto nombrado de pares key-value tipados que se inyectan automáticamente en templates al compilar. Son los datos de contexto — la información que no viene del evento sino de la identidad del workspace/tenant/global.

**Tipos de campo** soportados:

| Tipo | Descripción | Ejemplo de valor |
|---|---|---|
| `text` | Cadena de texto | "Acme Corporation" |
| `number` | Valor numérico | 42, 3.14 |
| `bool` | Verdadero/falso | true, false |
| `img` | URL de imagen | "https://cdn.acme.com/logo.png" |
| `url` | URL | "https://acme.com/soporte" |
| `html` | Bloque HTML sanitizado | "\<p>Footer legal\</p>" |

#### Modelo de Schema: Top-Down Fijo

El nivel que **crea** un inyector define su **schema** (nombres de campos y tipos). Esto es inmutable:

- Si el superadmin crea el inyector `brand` a nivel global con campos `{logo: img, name: text, email: text}`, ese es el contrato.
- Los niveles inferiores (`_system`, workspace) pueden **override valores** de esos campos, pero **NO pueden agregar campos nuevos ni eliminar campos existentes**.
- Si un `tenant-admin` necesita campos adicionales para su contexto, debe crear un **inyector nuevo** (ej: `brand-extra`) en su scope, no extender el inyector global.

Esto garantiza:
- Predicibilidad: un template que usa `{{ injector.brand.logo }}` sabe que el campo existe en todos los niveles.
- Validación: al compilar, si un campo referenciado no existe en el schema del inyector, es un error de template, no un problema de datos.
- Claridad: no hay "campos fantasma" que aparecen solo en algunos workspaces.

#### Herencia campo por campo

La resolución de campos sigue la cadena workspace → `_system` → global, campo por campo:
- Si el workspace define `brand.logo`, ese valor gana.
- Si no, se busca en `_system` del tenant.
- Si no, se busca en global.
- Los campos no overrideados se heredan del nivel superior.
- Un campo **no puede ponerse en null** para "borrar" un valor heredado. Si necesitas un campo vacío, usa un valor vacío del tipo correspondiente (ej: `""` para text, `0` para number).

**Diferencia con variables del evento:**
- **Inyectores:** Datos de contexto estáticos/semi-estáticos (marca, logos, URLs, datos de contacto). Resueltos automáticamente por Senda según la jerarquía.
- **Variables del evento:** Datos de instancia dinámicos (nombre del usuario, número de orden, monto). Enviados en cada request de la API por el servicio externo.

### 4.4. Ports y Adapters (Envío de Email)

Senda usa **arquitectura hexagonal** para el envío:

**Port (interfaz):** Contrato que cualquier proveedor de email debe cumplir.

**Adapter (implementación):** Implementación concreta para un proveedor. Inicialmente: SESAdapter y GmailAdapter.

Los adapters se configuran jerárquicamente (global → tenant `_system` → workspace) y siguen la misma cadena de resolución. Agregar nuevos adapters (SendGrid, Mailgun) requiere implementar un solo adapter sin tocar el core.

**Resolución de adapter:** Al enviar un email, Senda busca adapter en: workspace → `_system` → global. Si no existe en ningún nivel → `422 Unprocessable Entity`. Un workspace puede tener su propio adapter (ej: una cuenta SES específica) o heredar del tenant/global.

### 4.5. Sistema de Templates

#### Tipos de Plantilla

Un **tipo de plantilla** define un contrato: nombre (slug), descripción, y schema JSON de las variables del evento que espera. Los tipos se definen jerárquicamente:
- Tipos globales: disponibles para toda la plataforma.
- Tipos en `_system` del tenant: disponibles para todos los workspaces del tenant.
- Tipos en un workspace: solo disponibles en ese workspace.

La visibilidad sigue la misma cadena: un workspace puede usar tipos propios + tipos de su `_system` + tipos globales. Un workspace **puede crear tipos nuevos** que no existen en `_system` ni en global — estos tipos solo son visibles y usables dentro de ese workspace.

#### Plantillas y Versiones

Una **plantilla** es la implementación visual de un tipo. Cada plantilla tiene **versiones** con estados:

| Estado | Descripción |
|---|---|
| `draft` | En edición. Puede haber múltiples drafts. No se usa para envío. |
| `published` | Activa para envío. **Solo puede haber UNA versión publicada por plantilla.** |
| `archived` | Versión anterior preservada para historial. No se usa para envío. |

Al publicar un draft, la versión previamente publicada pasa automáticamente a `archived`.

#### Contenido del Template

Cada template contiene:

- **Body:** Código MJML (editado visual o manualmente) que define el contenido del email.
- **Subject:** Línea de asunto. Soporta variables de inyectores y del evento (ej: `"Bienvenido {{ event.user_name }} a {{ injector.brand.name }}"`).
- **Preview text:** Texto de preview que aparece en la bandeja de entrada después del subject. Soporta variables.
- **From address:** Dirección de remitente. Se compone de:
  - **From name:** Nombre visible (ej: "Soporte Acme"). Configurable por template, con fallback jerárquico. Soporta variables de inyectores.
  - **From email:** Email del remitente (ej: `soporte@acme.com`). Debe corresponder a un dominio verificado en la cadena de resolución. Configurable por template, con fallback jerárquico.
- **Reply-to:** (Opcional) Email de respuesta. Configurable por workspace/tenant/global con herencia.
- **Locale tags:** (Opcional) Etiquetas de idioma en bloques de texto para soporte i18n.

#### Regla de Unicidad por Scope

Dentro de cualquier scope (workspace, `_system`, o global), un tipo de plantilla **solo puede tener una plantilla asignada**. Esta regla aplica uniformemente en los tres niveles:

- Workspace "acme" tiene tipo "welcome" → solo puede haber UNA plantilla "welcome" en ese workspace.
- `_system` de "latam" tiene tipo "welcome" → solo puede haber UNA plantilla "welcome" en ese `_system`.
- Global tiene tipo "welcome" → solo puede haber UNA plantilla "welcome" a nivel global.

Esta regla es fundamental para el addressing determinístico.

#### Addressing Determinístico

Los emails se envían usando la referencia:

```
tenantCode:workspaceCode:templateType
```

Ejemplo: `latam:acme:welcome`

Esta referencia se resuelve así:
1. Busca el tenant con código `latam`.
2. Busca el workspace con código `acme` dentro de ese tenant.
3. Busca la plantilla asignada al tipo `welcome` en ese workspace.
4. Si no existe en el workspace, busca en `_system` del tenant `latam`.
5. Si no existe en `_system`, busca a nivel global.
6. Toma la versión `published` del template encontrado.
7. Si no hay template en ningún nivel → `404 Not Found`: `"No published template found for type 'welcome' in resolution chain of workspace 'acme', tenant 'latam'"`.

El servicio externo nunca necesita conocer IDs internos, versiones, ni estados. Solo necesita saber: tenant, workspace, y tipo de template. Senda resuelve el resto.

#### Desactivar Envío por Template

Un template puede ser **desactivado** (kill switch) sin archivarlo ni eliminarlo. Un template desactivado:
- Mantiene su versión `published` pero no se usa para envío.
- Si se intenta enviar un email con un template desactivado, Senda responde con `409 Conflict`: `"Template 'welcome' is disabled in workspace 'acme'"`.
- **Importante:** Si el template desactivado está en un nivel superior (`_system` o global) y un workspace no tiene override, el envío falla. Esto es intencional — el kill switch es una decisión administrativa que afecta hacia abajo.
- Es un mecanismo de emergencia para detener envíos sin perder la configuración.
- Reactivable en cualquier momento por un admin del scope correspondiente.

#### Variables en Templates

Los templates MJML pueden usar dos tipos de variables:

- Variables de **inyectores** (resueltas automáticamente por la jerarquía):
  ```
  {{ injector.brand.logo_url }}
  {{ injector.brand.company_name }}
  {{ injector.legal.footer_html }}
  ```

- Variables del **evento** (enviadas en el request de la API):
  ```
  {{ event.user_name }}
  {{ event.order_number }}
  {{ event.activation_url }}
  ```

Ambos tipos de variables pueden usarse en el **body**, **subject** y **preview text**.

### 4.6. Editor Visual

El editor de templates tiene dos modos complementarios:

**Drag-and-drop:** Arrastrar componentes (texto, imagen, botón, columnas, divider, spacer, social links) para construir la estructura.

**Inline editing:** Clic directo sobre el contenido del preview para editar texto, insertar variables, cambiar estilos. Edición WYSIWYG sobre el template renderizado.

Ambos modos trabajan sobre la misma representación interna (JSON Schema del editor ↔ MJML source). El usuario puede alternar entre modo visual y modo código (MJML directo).

#### i18n en Templates

Los bloques de texto del template (no inyectables, es decir, el contenido estático del template) pueden tener **configuración de idioma**:

- Cada bloque de texto puede marcarse con un **locale tag** (ej: `es`, `en`, `pt-BR`).
- Un template puede contener variantes de idioma para cada bloque de texto.
- El idioma se selecciona al momento del envío (el servicio externo indica `locale` en el request) o por configuración del workspace.
- Si no hay variante para el locale solicitado, se usa el locale por defecto del template.

**Integración con IA:**

El editor integra asistencia de IA para:
- **Armado de templates:** Sugerir estructura, copys y diseño basado en el tipo de email y contexto.
- **Traducciones:** Traducir automáticamente bloques de texto a los idiomas configurados.
- **Optimización de copy:** Mejorar textos para claridad, tono y engagement.
- **Consistencia:** Verificar que el tono y estilo sean coherentes entre variantes de idioma.

La integración de IA es una asistencia — el editor siempre retiene el control sobre el contenido final.

### 4.7. Flujo de Envío

1. Servicio externo llama a la API: `POST /api/v1/send` con:
   - `ref`: `"latam:acme:welcome"` (addressing determinístico)
   - `to`: array de destinatarios (máx 50 por request)
   - `variables`: variables del evento (user_name, order_number, etc.)
   - `external_id`: (opcional) ID del caso de negocio
   - `cc`: (opcional) array de emails en copia
   - `bcc`: (opcional) array de emails en copia oculta
   - `locale`: (opcional) código de idioma para i18n (ej: `"es"`, `"pt-BR"`)

2. Senda parsea la referencia → resuelve tenant, workspace, template type.
3. Senda verifica que el template no esté desactivado.
4. Senda resuelve la plantilla (workspace → `_system` → global) y toma la versión `published`.
5. Senda valida variables del evento contra el schema del tipo de plantilla.
6. Senda resuelve los inyectores (merge campo por campo: workspace → `_system` → global).
7. Senda resuelve el `from_email` usando la identidad default del adapter efectivo.
8. Senda valida que la identidad esté habilitada/verificada según el provider.
9. Senda resuelve subject y preview text (del template, aplicando variables).
10. Senda selecciona variante de idioma si `locale` fue proporcionado.
11. Senda compila MJML con inyectores + variables del evento → HTML responsive.
12. Senda encola como job transaccional (River/PostgreSQL). Un job por cada destinatario en `to`.
13. Worker envía vía el adapter configurado, respetando rate limits.
14. Senda registra resultado y actualiza lifecycle tracking. Cada destinatario recibe su propio `tracking_id`.
15. Si hay webhooks configurados, notifica al workspace/tenant.

#### Request y Response de la API

**Request:**
```json
POST /api/v1/send
Authorization: Bearer senda_live_abc123...

{
  "ref": "latam:acme:welcome",
  "to": ["user@example.com", "other@example.com"],
  "cc": ["manager@example.com"],
  "bcc": ["audit@internal.com"],
  "variables": {
    "user_name": "María García",
    "order_number": "ORD-12345",
    "activation_url": "https://app.acme.com/activate/xyz"
  },
  "external_id": "order_12345",
  "locale": "es"
}
```

**Response (202 Accepted):**
```json
{
  "status": "accepted",
  "tracking_ids": [
    {"to": "user@example.com", "tracking_id": "trk_a1b2c3d4"},
    {"to": "other@example.com", "tracking_id": "trk_e5f6g7h8"}
  ],
  "external_id": "order_12345",
  "template_resolved": "latam:acme:welcome",
  "template_version": 3
}
```

**Error Responses:**

| Code | Cuando | Ejemplo |
|---|---|---|
| `400` | Request malformado | Campos faltantes, JSON inválido |
| `401` | API Key inválida o revocada | |
| `403` | API Key no tiene acceso al workspace | |
| `404` | Template no encontrado en cadena | `"No published template for 'welcome' in chain"` |
| `409` | Template desactivado | `"Template 'welcome' is disabled in workspace 'acme'"` |
| `422` | Error de configuración o validación | No adapter, variables inválidas, dominio no verificado |
| `429` | Rate limit excedido | Retry-After header incluido |

### 4.8. External ID y Trazabilidad

Cada email puede llevar un **external_id**: identificador del sistema de negocio (ej: order_12345, ticket_789). El campo es **opcional** — si no se envía, el email se trackea solo por su `tracking_id` interno.

Permite:
- Consultar todos los emails de un caso de negocio. **Scope por rol:** workspace-admin/editor/viewer solo ven external_ids de su workspace. Tenant-admin ve de todos sus workspaces. Superadmin ve cross-workspace y cross-tenant.
- Búsqueda por email de destinatario/remitente (mismo scope por rol).
- Exportación masiva con filtros.
- Paginación cursor-based en todas las APIs de consulta.
- API Keys: solo retornan datos del workspace al que pertenecen.

### 4.9. Email Lifecycle Tracking

Estados del ciclo de vida:

| Estado | Descripción |
|---|---|
| `queued` | Request recibido, job encolado |
| `processing` | Worker compilando y preparando envío |
| `sent` | Entregado al proveedor (SES/Gmail aceptó) |
| `delivered` | Proveedor confirmó entrega al destino |
| `opened` | Destinatario abrió el email (solo si tracking activo) |
| `bounced` | Rebote (soft o hard) |
| `complained` | Marcado como spam |
| `failed` | Error en el envío |
| `suppressed` | Dirección en suppression list, email no enviado |

#### Open Tracking (Pixel de Apertura)

El tracking de apertura está **desactivado por defecto** y se activa **opt-in por workspace**:

- Cada `workspace-admin` decide si activar open tracking para su workspace.
- Si está activo, Senda inserta automáticamente un pixel de tracking en el email.
- Se añade automáticamente un disclaimer configurable al footer del email cuando el tracking está activo.
- El estado `opened` solo se registra si el tracking está activo en el workspace del email.
- Consideraciones GDPR: el disclaimer informa al destinatario. Cada empresa es responsable de cumplir con su regulación local.

### 4.10. Miembros y Roles (Autorización)

Senda separa **autenticación** (quién eres) de **autorización** (qué puedes hacer):

- **Autenticación:** Delegada a OIDC externo (Google Workspace, Keycloak, etc.). Senda no gestiona contraseñas ni identidades.
- **Autorización:** Gestionada internamente por Senda. Un usuario autenticado por OIDC solo accede a Senda si está registrado como **miembro**.

#### Miembros

Un **miembro** es un registro en Senda identificado por su email. Para acceder al dashboard:
1. El usuario se autentica contra el proveedor OIDC.
2. Senda verifica que el email del token OIDC existe como miembro registrado.
3. Si no existe → acceso denegado (incluso si OIDC pasó correctamente): `"Acceso denegado. Tu email no está registrado como miembro. Contacta a tu administrador."`.
4. Si existe → Senda carga los roles asignados al miembro.

Los miembros se agregan por invitación (un admin registra el email). No existe auto-registro.

#### Roles

Un miembro puede tener **diferentes roles en diferentes scopes**. Los roles son fijos (no customizables):

| Rol | Scope | Capacidades |
|---|---|---|
| `superadmin` | Global | Acceso total. Gestiona toda la plataforma: tenants, configuración global, dominios, miembros en cualquier nivel. |
| `tenant-admin` | Tenant específico | Gestiona el tenant: `_system` workspace, workspaces, inyectores, templates, adapters, dominios, y miembros dentro de su tenant. |
| `workspace-admin` | Workspace específico | Gestiona el workspace: inyectores, templates, dominios, webhooks, y miembros del workspace. |
| `workspace-editor` | Workspace específico | Edita templates e inyectores del workspace. No gestiona miembros, dominios ni adapters. |
| `workspace-viewer` | Workspace específico | Solo lectura. Ve templates, métricas y estado de emails del workspace. No puede modificar nada. |

**Múltiples superadmins:** Puede haber múltiples superadmins. El superadmin original (creado en onboarding) puede crear otros superadmins. Esto es útil para empresas donde más de una persona necesita acceso total a la plataforma.

**Roles múltiples:** Un miembro puede ser `tenant-admin` en el tenant "latam" y `workspace-viewer` en un workspace del tenant "europe". Los roles no se heredan — ser `tenant-admin` no otorga automáticamente `workspace-admin` en los workspaces del tenant, sino que el `tenant-admin` tiene acceso a todos los workspaces de su tenant como parte de su propio rol.

**Tabla de permisos:**

| Acción | superadmin | tenant-admin | ws-admin | ws-editor | ws-viewer |
|---|---|---|---|---|---|
| Crear/gestionar tenants | ✓ | — | — | — | — |
| Config global (adapters, inyectores, templates, dominios) | ✓ | — | — | — | — |
| Agregar miembros a cualquier nivel | ✓ | — | — | — | — |
| Crear otros superadmins | ✓ | — | — | — | — |
| Config `_system` del tenant | ✓ | ✓ | — | — | — |
| Crear/gestionar workspaces del tenant | ✓ | ✓ | — | — | — |
| Gestionar dominios del tenant | ✓ | ✓ | — | — | — |
| Agregar miembros al tenant y sus workspaces | ✓ | ✓ | — | — | — |
| Ver métricas del tenant (todos los workspaces) | ✓ | ✓ | — | — | — |
| Config workspace (adapters, dominios, webhooks) | ✓ | ✓ | ✓ | — | — |
| Agregar miembros al workspace | ✓ | ✓ | ✓ | — | — |
| Gestionar API Keys del workspace | ✓ | ✓ | ✓ | — | — |
| Editar templates e inyectores del workspace | ✓ | ✓ | ✓ | ✓ | — |
| Publicar/archivar/desactivar templates | ✓ | ✓ | ✓ | — | — |
| Ver templates, métricas, estado de emails | ✓ | ✓ | ✓ | ✓ | ✓ |
| Enviar test emails desde dashboard | ✓ | ✓ | ✓ | ✓ | — |

Nota: `workspace-editor` puede crear y editar drafts de templates, pero **no puede publicarlos** — eso requiere `workspace-admin` o superior. Esto permite un flujo de revisión donde el editor prepara y el admin aprueba.

#### Onboarding Inicial

Cuando Senda se instala por primera vez y la base de datos está vacía:

1. El primer usuario que completa el login OIDC se registra automáticamente como **superadmin**.
2. Se crea el primer tenant (el sistema pide código y nombre).
3. Se crea el workspace `_system` automáticamente.
4. Se crea el primer workspace regular (el sistema pide código y nombre).
5. A partir de aquí, el superadmin gestiona todo manualmente.

Este flujo solo ocurre una vez. Si ya existe al menos un miembro, el login OIDC valida contra la tabla de miembros normalmente.

### 4.11. API Keys

El acceso a la API de envío y consulta (plano de datos) no usa OIDC sino **API Keys** de larga duración:

- Cada API Key pertenece a un workspace específico.
- Formato con prefijo identificable: `senda_live_<random>` (producción) y `senda_test_<random>` (sandbox).
- Se almacenan hasheadas en la base de datos (nunca texto plano).
- Un workspace puede tener múltiples API Keys activas.
- Las API Keys se pueden revocar individualmente.
- Una API Key solo puede enviar emails y consultar datos dentro de su workspace (y la resolución jerárquica aplica desde ese workspace).

**Operaciones permitidas con API Key:**
- Enviar emails (`POST /api/v1/send`)
- Consultar estado de email por tracking_id
- Consultar emails por external_id
- Buscar emails por destinatario/remitente
- Exportar registros con filtros

**Modo test/sandbox:** La funcionalidad de test y simulación (enviar un email de prueba sin entregarlo realmente) se realiza desde el **dashboard** autenticado vía OIDC, no desde la API. Esto permite que editores y admins prueben templates sin necesidad de API Keys de test.

### 4.12. Soft Delete y Gestión de Dependencias

Cuando un admin elimina un recurso (template, inyector, adapter) que podría estar siendo heredado por niveles inferiores:

**Soft delete:** El recurso no se borra físicamente. Se marca como `deleted` con timestamp.

**Comportamiento:**
- El recurso sigue disponible en la cadena de resolución para los scopes que lo heredaban.
- Los scopes que lo heredan ven un **warning visual** en el dashboard: `"Este recurso está marcado como deprecado en [nivel]. Se recomienda configurar un override."`.
- Los envíos siguen funcionando normalmente — el soft delete no interrumpe operaciones.
- Un admin puede hacer **purge** (eliminación definitiva) después de verificar que no hay dependencias, o que los dependientes ya tienen overrides.

**Antes del purge**, el sistema muestra:
- Lista de scopes que heredan el recurso sin override propio.
- Impacto estimado: cuántos workspaces/templates quedarían afectados.
- Requiere confirmación explícita.

**Audit trail:** Todas las operaciones de soft delete y purge se registran con: quién, cuándo, qué recurso, y estado de dependencias al momento de la acción.

### 4.13. Identidades de Envío (Provider-Managed)

Senda usa modelo **provider-managed** para autenticación de email:

- SPF/DKIM/DMARC son responsabilidad del proveedor (SES/Gmail).
- Senda no genera ni firma DKIM en aplicación.
- Senda sincroniza y valida identidades disponibles en el provider para cada adapter.

**Flujo:**
1. Admin configura adapter SES/Gmail en el scope correspondiente.
2. Senda sincroniza identidades desde provider (emails/dominios verificados, estado de envío).
3. Admin selecciona identidad default por adapter.
4. `POST /send` usa la identidad default efectiva del adapter resuelto.

**Regla de envío:** si el adapter no tiene identidad default válida/verificada, el envío falla con error funcional (`422`).

### 4.14. Suppression Lists

Senda mantiene dos niveles de suppression:

**Suppression global (hard bounces):**
- Un email que genera hard bounce se añade automáticamente a la suppression list global.
- Bloquea envíos a esa dirección en **todos los workspaces** de toda la plataforma.
- Solo un `superadmin` puede remover una dirección de la suppression global (con justificación registrada).

**Suppression por workspace (complaints):**
- Si un destinatario marca como spam un email de un workspace específico, se suprime solo para ese workspace.
- El `workspace-admin` puede ver su lista de suppressions.
- No afecta otros workspaces (un complaint en "acme" no suprime en "beta").

**Interacción:**
- Al enviar, Senda verifica ambas listas. Si el email está en cualquiera → estado `suppressed`, email no enviado.
- La suppression global es absoluta: un workspace-admin NO puede des-suprimir una dirección que está en la lista global.

**Soft bounces:**
- Soft bounce → retry automático (máx 3 intentos con backoff exponencial).
- Después de 3 intentos fallidos → estado `failed` (no se suprime automáticamente).
- Si un email genera 3+ soft bounces en 7 días → alerta al workspace-admin.

**Alertas:**
- Si bounce rate > 5% en 24h para un workspace → alerta al workspace-admin y tenant-admin.
- Si complaint rate > 0.1% → alerta inmediata.

### 4.15. Audit Logging

Senda registra un **audit log** de todas las acciones administrativas:

- **Quién:** Email del miembro que realizó la acción.
- **Cuándo:** Timestamp UTC.
- **Qué:** Acción realizada (create, update, delete, publish, disable, etc.).
- **Dónde:** Scope (global, tenant, workspace).
- **Detalle:** Cambios específicos (ej: "field 'logo' changed from 'old.png' to 'new.png'").

El audit log es append-only (no se puede modificar ni eliminar). Visible para admins del scope correspondiente y superadmins.

---

## 5. User Stories

### Administrador Global (Superadmin)

**US-01:** Como superadmin, quiero definir inyectores base a nivel global con su schema (campos y tipos) para que todos los tenants y workspaces hereden esos valores como default.

**US-02:** Como superadmin, quiero definir tipos de plantilla globales con su schema de variables para establecer estándares de datos validados.

**US-03:** Como superadmin, quiero crear plantillas globales para cada tipo (incluyendo subject, preview text y from address) para que workspaces sin personalización tengan un diseño base funcional.

**US-04:** Como superadmin, quiero configurar adapters de envío (SES, Gmail) globales como transporte por defecto.

**US-05:** Como superadmin, quiero ver métricas agregadas de toda la plataforma.

**US-06:** Como superadmin, quiero gestionar adapters e identidades verificadas del provider a nivel global.

**US-07:** Como superadmin, quiero configurar parámetros globales (rate limits, reintentos, retención de logs).

**US-08:** Como superadmin, quiero crear tenants con códigos únicos y gestionar su configuración base.

**US-09:** Como superadmin, quiero agregar miembros (por email) y asignarles roles en cualquier nivel (global, tenant, workspace), incluyendo crear otros superadmins.

**US-10:** Como superadmin, quiero que al instalar Senda por primera vez, el primer login OIDC me registre automáticamente como superadmin y me guíe para crear el primer tenant y workspace.

**US-11:** Como superadmin, quiero ver el audit log de acciones administrativas de toda la plataforma.

### Administrador de Tenant

**US-12:** Como admin de tenant, quiero configurar templates, inyectores y adapters en el workspace `_system` para que sean heredados por todos mis workspaces.

**US-13:** Como admin de tenant, quiero crear workspaces con códigos únicos (dentro de mi tenant) para organizar mis clientes/marcas/productos.

**US-14:** Como admin de tenant, quiero crear tipos de plantilla adicionales en `_system` propios de mi contexto de negocio.

**US-15:** Como admin de tenant, quiero ver métricas agregadas de todos mis workspaces.

**US-16:** Como admin de tenant, quiero gestionar identidades de envío del provider a nivel tenant (`_system`) para que mis workspaces las hereden.

**US-17:** Como admin de tenant, quiero agregar miembros a mi tenant y a los workspaces de mi tenant, asignándoles roles apropiados.

**US-18:** Como admin de tenant, quiero soft-deletear recursos en `_system` y ver qué workspaces serían afectados antes de hacer purge.

### Administrador de Workspace

**US-19:** Como admin de workspace, quiero overridear valores de inyectores heredados (campo por campo, sin modificar el schema) para personalizar la identidad de mis emails.

**US-20:** Como admin de workspace, quiero crear plantillas propias para tipos existentes, sabiendo que solo puedo tener una plantilla por tipo, con subject, preview text y from address.

**US-21:** Como admin de workspace, quiero gestionar versiones de mis templates: crear drafts, publicar (reemplazando la versión publicada anterior), y ver historial de versiones archivadas.

**US-22:** Como admin de workspace, quiero desactivar un template para detener envíos de emergencia sin perder la configuración.

**US-23:** Como admin de workspace, quiero configurar webhooks para recibir notificaciones de cambio de estado de emails.

**US-24:** Como admin de workspace, quiero ver métricas exclusivas de mi workspace.

**US-25:** Como admin de workspace, quiero agregar miembros a mi workspace con roles de editor o viewer.

**US-26:** Como admin de workspace, quiero gestionar API Keys de mi workspace: crear, ver (parcialmente ocultas), y revocar.

**US-27:** Como admin de workspace, quiero activar/desactivar open tracking para mi workspace.

**US-28:** Como admin de workspace, quiero enviar test emails desde el dashboard para probar templates antes de publicar.

### Servicio Externo (API Consumer)

**US-29:** Como servicio externo, quiero enviar un email vía API usando la referencia `tenantCode:workspaceCode:templateType` con un array de hasta 50 destinatarios, y recibir un 202 Accepted con tracking_ids.

**US-30:** Como servicio externo, quiero consultar el estado de un email por tracking_id.

**US-31:** Como servicio externo, quiero consultar todos los emails de un external_id con paginación.

**US-32:** Como servicio externo, quiero buscar emails por dirección de destinatario o remitente con paginación.

**US-33:** Como servicio externo, quiero exportar masivamente registros de email con filtros.

**US-34:** Como servicio externo, quiero recibir un error claro si intento enviar con un template desactivado (409 Conflict).

**US-35:** Como servicio externo, quiero especificar un `locale` en el request para seleccionar la variante de idioma del template.

### Workspace Editor

**US-36:** Como editor de workspace, quiero crear y editar drafts de templates usando el editor visual o código MJML, incluyendo subject y preview text.

**US-37:** Como editor de workspace, quiero editar texto directamente sobre el preview (inline editing).

**US-38:** Como editor de workspace, quiero previsualizar en desktop y mobile.

**US-39:** Como editor de workspace, quiero insertar variables de inyectores y del evento desde un selector visual.

**US-40:** Como editor de workspace, quiero guardar como draft, sabiendo que un admin debe publicarlo.

**US-41:** Como editor de workspace, quiero configurar variantes de idioma para bloques de texto del template.

**US-42:** Como editor de workspace, quiero usar asistencia de IA para traducir bloques de texto y mejorar el copy.

**US-43:** Como editor de workspace, quiero enviar test emails para verificar cómo se ve el template.

### Miembros

**US-44:** Como usuario con email registrado como miembro, quiero autenticarme vía OIDC y acceder automáticamente a los tenants y workspaces donde tengo rol asignado.

**US-45:** Como usuario no registrado como miembro, quiero recibir un mensaje claro de "acceso denegado, contacta a tu administrador" al intentar acceder después de autenticarme por OIDC.

---

## 6. Requirements

### Must-Have (P0)

**R-01: Jerarquía de 3 niveles con herencia y `_system`**
Global → Tenant → Workspace. Cada tenant tiene un workspace `_system` auto-creado para configuración heredable. Resolución: workspace → `_system` del tenant → global.
- [x] Aislamiento: recursos de workspace A no visibles desde workspace B (excepto herencia de `_system`/global).
- [x] `_system` se crea automáticamente al crear un tenant. No se puede eliminar ni renombrar.
- [x] No se pueden enviar emails desde `_system`.
- [x] Si workspace no define recurso, hereda de `_system`; si `_system` no lo define, hereda de global.
- [x] Si recurso no existe en ningún nivel → error claro (422).
- [x] Mínimo: 1 tenant + 1 workspace (además de `_system`).
- [x] Cross-tenant isolation: roles no-superadmin no tienen visibilidad fuera de su scope.

**R-02: Códigos (slugs) para tenants y workspaces**
Identificadores humanos en formato slug.
- [x] Formato: `[a-z][a-z0-9-]*`, 2-50 chars, sin `--`, sin guion final.
- [x] Tenant code: único global.
- [x] Workspace code: único dentro de su tenant.
- [x] Reservados: `_system`, `global`, `admin`, `api`, `system`.
- [x] Inmutables después de creación (para no romper integraciones externas).

**R-03: Inyectores tipados con schema top-down fijo y herencia campo por campo**
Conjuntos nombrados de key-value tipados. Schema definido por el nivel creador; niveles inferiores solo override valores.
- [x] Tipos validados: text, number, bool, img, url, html.
- [x] Schema fijo: el nivel que crea el inyector define campos + tipos. Niveles inferiores no pueden agregar ni eliminar campos.
- [x] Override parcial: workspace overridea solo los valores que necesita.
- [x] No se puede poner un campo en null (usar valor vacío del tipo correspondiente).
- [x] Si un nivel necesita campos extra, crea un inyector nuevo con otro nombre.
- [x] Validación: en compilación, si un template referencia un campo que no existe en el schema → error.

**R-04: API de envío con addressing determinístico**
`POST /api/v1/send` con referencia `tenantCode:workspaceCode:templateType`.
- [x] Parsea referencia → resuelve tenant, workspace, template.
- [x] `to`: array de emails, máximo 50 por request. Cada uno genera un email independiente con tracking_id propio.
- [x] `cc` y `bcc`: opcionales, arrays de emails.
- [x] `external_id`: opcional.
- [x] `locale`: opcional, para seleccionar variante de idioma.
- [x] Valida variables contra schema del tipo.
- [x] Verifica template no desactivado.
- [x] Verifica dominio verificado para el from address.
- [x] Respuesta < 100ms p99 con tracking_ids.
- [x] 400 malformado, 401 API Key inválida, 403 sin acceso, 404 template no encontrado, 409 template desactivado, 422 error de config/validación, 429 rate limit.

**R-05: Templates MJML con subject, preview text, from address y resolución jerárquica**
MJML compilado vía gomjml. Variables de inyectores + evento en body, subject y preview text.
- [x] Compilación < 10ms p99.
- [x] XSS prevention en variables.
- [x] Subject y preview text soportan variables de inyectores y evento.
- [x] From address: from_name + from_email configurables por template con fallback jerárquico.
- [x] From email debe corresponder a dominio verificado en la cadena.
- [x] Reply-to configurable por workspace/tenant/global con herencia.
- [x] Renderiza en Gmail, Outlook, Apple Mail.

**R-06: Tipos de plantilla con schema de variables**
Contrato: nombre (slug), descripción, schema JSON del evento.
- [x] Validación de variables al enviar.
- [x] Visibilidad jerárquica (global, `_system`, workspace).

**R-07: Versionado de templates con estados**
draft → published → archived. Solo UNA versión published por template.
- [x] Múltiples drafts permitidos.
- [x] Al publicar: draft → published, published anterior → archived.
- [x] Solo la versión published se usa para envío.
- [x] Historial de versiones archivadas visible.
- [x] Revertir: crear nuevo draft desde una versión archivada.

**R-08: Unicidad de tipo por scope**
Un tipo de plantilla solo puede tener UNA plantilla asignada dentro de un scope (workspace, `_system`, o global).
- [x] Aplica uniformemente en los tres niveles.
- [x] Al intentar crear segunda plantilla del mismo tipo en un scope → error.
- [x] Esto garantiza que el addressing determinístico siempre resuelve a exactamente un template.

**R-09: Desactivar template (kill switch)**
Posibilidad de desactivar un template sin archivarlo.
- [x] Template desactivado mantiene su versión published pero no se envía.
- [x] API retorna 409 Conflict con mensaje claro.
- [x] Reactivable en cualquier momento por admin del scope.
- [x] Si desactivado en nivel superior y workspace no tiene override, envío falla.
- [x] Estado visible en dashboard.

**R-10: Ports y Adapters para envío (SES + Gmail)**
Arquitectura hexagonal. Port define contrato, adapters lo implementan.
- [x] SESAdapter: region, access key, secret key (encriptados).
- [x] GmailAdapter: OAuth credentials (encriptados).
- [x] Herencia jerárquica de adapters.
- [x] Si no hay adapter en ningún nivel → 422 con mensaje descriptivo.
- [x] Extensible sin modificar core.

**R-11: Email lifecycle tracking**
queued → processing → sent → delivered → opened → bounced → complained → failed → suppressed.
- [x] Consulta por tracking_id.
- [x] Historial con timestamps.
- [x] Webhooks del proveedor actualizan automáticamente.
- [x] `opened` solo disponible si tracking está activo en el workspace.

**R-12: Trazabilidad por external_id**
- [x] Campo opcional en el request de envío.
- [x] Consulta paginada con filtros.
- [x] Indexado para eficiencia.
- [x] Cross-workspace/cross-tenant para superadmin.

**R-13: Autenticación de email (provider-managed)**
- [x] SPF/DKIM/DMARC delegados al provider (SES/Gmail).
- [x] Senda valida identidad default del adapter antes de enviar.
- [x] Si identidad no verificada/no habilitada, envío falla.
- [x] Senda no implementa firma DKIM in-app.

**R-14: Sincronización de identidades con herencia**
- [x] Sync manual/API de identidades por adapter.
- [x] Estados de identidad reflejan estado del provider.
- [x] Identidad default configurable por scope.
- [x] Herencia de adapter/identidad: workspace → `_system` → global.
- [x] Fallback explícito para adapters sin listado de identidades (ej. SMTP/manual).

**R-15: Bounce y complaint handling con suppression lists**
- [x] Suppression global (hard bounces): bloquea en TODOS los workspaces. Solo superadmin puede remover.
- [x] Suppression por workspace (complaints): solo afecta ese workspace.
- [x] Al enviar: verifica ambas listas. Si está en cualquiera → `suppressed`.
- [x] Soft bounce → retry (máx 3, backoff exponencial). Después → `failed`.
- [x] Alerta si bounce > 5% en 24h por workspace.
- [x] Alerta si complaint > 0.1%.

**R-16: Dashboard con métricas**
- [x] Vistas: global, por tenant, por workspace.
- [x] Tendencias: 7d, 30d, 90d.
- [x] Detalle de emails individuales con lifecycle.

**R-17: Autenticación vía OIDC + Membresía**
Autenticación delegada a OIDC. Autorización gestionada por Senda via membresía.
- [x] Configuración de proveedor OIDC con discovery URL.
- [x] Post-OIDC: Senda verifica que el email existe como miembro registrado.
- [x] Si no es miembro → acceso denegado con mensaje claro.
- [x] Si es miembro → carga roles y scopes del miembro.

**R-18: Sistema de miembros y roles**
Miembros registrados por email. Roles fijos con scopes jerárquicos.
- [x] Roles: superadmin, tenant-admin, workspace-admin, workspace-editor, workspace-viewer.
- [x] Múltiples superadmins permitidos.
- [x] Un miembro puede tener diferentes roles en diferentes scopes.
- [x] Superadmin puede agregar miembros y asignar cualquier rol en cualquier nivel, incluyendo crear otros superadmins.
- [x] Tenant-admin puede agregar miembros a su tenant y sus workspaces.
- [x] Workspace-admin puede agregar miembros a su workspace (roles: editor, viewer).
- [x] La tabla de permisos (sección 4.10) se aplica estrictamente en cada endpoint.
- [x] workspace-editor puede crear/editar drafts pero NO puede publicar templates.

**R-19: Onboarding inicial**
Primer usuario se convierte en superadmin automáticamente.
- [x] Si la DB está vacía y un usuario completa OIDC → se registra como superadmin.
- [x] El sistema guía al superadmin para crear primer tenant (código + nombre) y primer workspace.
- [x] `_system` se crea automáticamente con el primer tenant.
- [x] Este flujo solo ocurre una vez (si ya hay miembros, login normal).

**R-20: API Keys para plano de datos**
Autenticación de la API de envío y consulta via API Keys de larga duración.
- [x] Cada API Key pertenece a un workspace.
- [x] Formato: `senda_live_<random>` (prod). Solo se muestra completa al momento de creación.
- [x] Almacenadas hasheadas (nunca texto plano).
- [x] Un workspace puede tener múltiples API Keys.
- [x] Revocación individual.
- [x] API Key permite: enviar, consultar estado, buscar emails, exportar. Solo dentro de su workspace.

**R-21: API de consulta y búsqueda**
- [x] Por external_id, email, filtros combinados.
- [x] Paginación cursor-based.
- [x] Exportación masiva.
- [x] Scoped por API Key (solo datos del workspace).

**R-22: Soft delete y gestión de dependencias**
- [x] Recursos eliminados se marcan como `deleted` (soft delete).
- [x] Siguen disponibles en herencia, con warning visual.
- [x] Purge requiere: ver lista de dependencias, confirmar explícitamente.
- [x] Audit trail de deletes y purges.

**R-23: Audit logging**
- [x] Registro de todas las acciones administrativas.
- [x] Quién, cuándo, qué, dónde, detalle de cambios.
- [x] Append-only (no modificable).
- [x] Visible por admins del scope y superadmins.

### Nice-to-Have (P1)

**R-24: Editor visual drag-and-drop + inline con i18n**
- [ ] Componentes arrastrables.
- [ ] Inline editing en preview.
- [ ] Selector visual de variables.
- [ ] Alternancia modo visual / modo código.
- [ ] Bloques de texto con locale tags para variantes de idioma.
- [ ] Selector de locale por defecto del template.

**R-25: Integración con IA en editor**
- [ ] Asistencia para armado de templates (sugerir estructura y copy).
- [ ] Traducción automática de bloques de texto.
- [ ] Optimización de copy (claridad, tono, engagement).
- [ ] Verificación de consistencia entre variantes de idioma.

**R-26: Open tracking (opt-in por workspace)**
- [ ] Desactivado por defecto.
- [ ] Workspace-admin activa/desactiva.
- [ ] Pixel de tracking insertado automáticamente.
- [ ] Disclaimer configurable en footer cuando activo.

**R-27: Webhooks de eventos**
- [ ] Configuración por workspace.
- [ ] Retry con backoff exponencial (máx 5 intentos).
- [ ] Firma HMAC-SHA256 para verificación.
- [ ] Eventos: sent, delivered, opened, bounced, complained, failed.
- [ ] Payload: `{ event, tracking_id, external_id, workspace, tenant, timestamp, data }`.

**R-28: Rate limiting por adapter**
- [ ] Token bucket distribuido (Redis).
- [ ] Configurable por adapter, tenant, workspace.
- [ ] Backpressure sin fallar: encola y espera, no descarta.
- [ ] Nota: rate limiting básico para respetar límites de SES/Gmail debe estar en Fase 1.

**R-29: Test emails desde dashboard**
- [ ] Enviar email de prueba desde el editor (autenticado vía OIDC, no API Key).
- [ ] Ejecuta pipeline completo excepto: no registra en lifecycle real, marca como "test".
- [ ] Destinatario: el email del miembro que hace el test u otro especificado.

### Future Considerations (P2)

**R-30:** SMTP Relay.
**R-31:** Adapters adicionales (SendGrid, Mailgun, Postmark).
**R-32:** A/B testing de templates.
**R-33:** Scheduling de envíos.
**R-34:** Template sharing entre workspaces.
**R-35:** Roles customizables (RBAC completo con permisos configurables por instalación).
**R-36:** Batch endpoint `/api/v1/send/batch` para envíos masivos (hasta 10K destinatarios, retorna batch_id).

---

## 7. Stack Tecnológico

### Backend

| Componente | Tecnología | Justificación |
|---|---|---|
| Lenguaje | Go 1.23+ | Performance, binario estático, concurrencia nativa |
| Web Framework | Echo 4.x | HTTP/2, middleware net/http, ecosistema maduro |
| Base de datos | PostgreSQL 16+ | ACID, RLS, partitioning |
| Job Queue | River (PostgreSQL) | Transaccional, ~10K jobs/seg, Web UI |
| Templates | gomjml (pure Go) | ~3ms compile, ~2MB RAM |
| Email Auth (SPF/DKIM/DMARC) | Provider-managed (SES/Gmail) | Evita duplicar firma/validación en app |
| Cache/Rate Limit | PostgreSQL (UNLOGGED + PL/pgSQL) | Sin Redis, operación unificada en PG |

### Frontend

| Componente | Tecnología |
|---|---|
| Core | React 19 + React Compiler |
| Build | Vite 6 |
| Estilos | Tailwind CSS v4 |
| Estado | TanStack Query v5 + Zustand |

### Deployment

Docker + Docker Compose. Caddy opcional para HTTPS.

### Diagrama de Arquitectura

```
                         ┌──────────────────────────┐
   Servicio Externo ───► │     REST API (Echo)       │
                         │  tenantCode:wsCode:type   │
                         └────────────┬─────────────┘
                                      │
                         ┌────────────▼─────────────┐
                         │    Application Core       │
                         │                           │
                         │  ┌─ Hierarchy Resolver    │
                         │  ├─ Injector Resolver     │
                         │  ├─ Template Engine        │
                         │  ├─ Lifecycle Tracker      │
                         │  ├─ Suppression Manager   │
                         │  ├─ Domain Verifier       │
                         │  └─ Audit Logger          │
                         └────────────┬─────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
     ┌────────▼────────┐   ┌─────────▼────────┐   ┌─────────▼────────┐
     │  Port: Sender    │   │  Port: Store     │   │  Port: Queue     │
     │  ┌─ SESAdapter   │   │  ┌─ PostgreSQL   │   │  ┌─ River        │
     │  └─ GmailAdapter │   │  └─              │   │  └─              │
     └─────────────────┘   └──────────────────┘   └──────────────────┘
```

---

## 8. Success Metrics

### Leading

| Métrica | Target |
|---|---|
| API response time (envío) | < 100ms p99 |
| Tasa de entrega | > 98% |
| Compilación template | < 10ms p99 |
| Resolución inyectores | < 5ms p99 |
| Resolución addressing | < 2ms p99 |

### Lagging

| Métrica | Target |
|---|---|
| Bounce rate | < 2% |
| Complaint rate | < 0.1% |

---

## 9. Open Questions

### Blocking

**OQ-01 [Engineering]:** ~~Open tracking.~~ **RESUELTO:** Opt-in por workspace. Desactivado por defecto. Disclaimer automático cuando activo.

**OQ-02 [Engineering]:** Editor visual: ¿custom, GrapeJS, Unlayer, u otro OS? Decisión diferida a Fase 3 (editor visual).

**OQ-03 [Product]:** ~~Mapeo OIDC → tenant/workspace~~ **RESUELTO:** El mapeo es por membresía explícita. Un admin registra el email del miembro y le asigna roles en scopes específicos. No se mapea automáticamente por dominio ni claim.

**OQ-04 [Engineering]:** Gmail: 2K msg/día/cuenta. ¿Rotación? ¿Documentar limitación? → Documentar como limitación conocida del GmailAdapter. Rotación de cuentas como feature futura.

**OQ-05 [Product]:** ~~external_id: ¿obligatorio u opcional?~~ **RESUELTO:** Opcional. Si no se envía, el email se trackea solo por tracking_id interno.

**OQ-06 [Engineering]:** ~~Schema de inyectores~~ **RESUELTO:** Schema top-down fijo. El nivel que crea el inyector define campos + tipos. Niveles inferiores solo override valores, no pueden agregar ni eliminar campos.

### Non-Blocking

**OQ-07:** Partitioning de PostgreSQL (mes + tenant).
**OQ-08:** RLS adicional o solo en aplicación.
**OQ-09:** Dashboard: SPA o server-rendered.
**OQ-10:** Integración IA: ¿qué modelo/proveedor? ¿Self-hosted o API externa?
**OQ-11:** API versioning: ¿URL prefix (/v1/, /v2/) o header? Recomendado: URL prefix.

---

## 10. Phasing

### Fase 1 — Core
Jerarquía 3 niveles + `_system`, códigos slug, inyectores tipados (schema top-down), API con addressing determinístico (array to, cc, bcc, locale), templates MJML code-based (con subject, preview text, from address), versionado con estados, kill switch, lifecycle tracking, external_id (opcional), SES/Gmail adapters, autenticación de email provider-managed (sin DKIM in-app), miembros y roles (múltiples superadmins), onboarding inicial, API Keys (send + query), OIDC, suppression lists (global + workspace), soft delete + dependencias, audit logging, rate limiting básico (respetar límites SES), soporte básico de i18n (campo `locale` en API + resolución de variante en template).

### Fase 2 — Observabilidad
Dashboard, métricas, búsqueda avanzada, exportación masiva, alertas de bounce/complaint, test emails desde dashboard.

### Fase 3 — UX
Editor visual drag-and-drop + inline, gestión visual de variantes de idioma en editor, integración IA (traducción + armado de templates + copy), webhooks de eventos.

### Fase 4 — Hardening
Rate limiting avanzado (token bucket distribuido), Gmail adapter, open tracking (opt-in), optimización de performance, documentación OS.

---

*Draft para iteración. Nada es versión final.*
