# Design Brief — Senda Dashboard

**Para:** Equipo UX/UI

**Referencia:** PRD v5.0 | TECH_SPEC v1.4

**Plataforma:** Web responsive, mobile-first

**Stack UI:** Tailwind CSS + shadcn/ui

**Auth:** OIDC externo (Google Workspace, Keycloak, etc.)

---

## 1. Resumen del Producto

Senda es una plataforma open-source de orquestación de email transaccional. El dashboard permite a administradores gestionar una jerarquía de 3 niveles (Global → Tenant → Workspace), configurar templates de email con herencia, gestionar dominios, adapters de envío, y monitorear el ciclo de vida completo de cada email.

**Público del dashboard:** Administradores técnicos y semi-técnicos de empresas que gestionan múltiples marcas/regiones/clientes. No es un producto consumer — es un tool interno de operaciones.

---

## 2. Roles y Permisos (Impacta toda la UI)

Cada pantalla debe respetar los permisos del rol activo. Los elementos no permitidos se **ocultan** (no se deshabilitan).

| Rol | Scope | Ve | Puede mutar |
|-----|-------|-----|-------------|
| **superadmin** | Global | Todo | Todo |
| **tenant_admin** | Su tenant | Todo del tenant + heredados de global | Config _system, workspaces, miembros del tenant |
| **workspace_admin** | Su workspace | Todo del workspace + heredados | Config workspace, miembros, API keys, webhooks |
| **workspace_editor** | Su workspace | Todo del workspace | Templates (draft only), inyectores |
| **workspace_viewer** | Su workspace | Todo del workspace | Nada (solo lectura) |

**Múltiples scopes:** Un usuario puede tener roles en distintos tenants/workspaces. El dashboard debe permitir navegar entre scopes.

---

## 3. Information Architecture

### 3.1. Navegación Principal

```
┌──────────────────────────────────────────────────────┐
│  HEADER                                              │
│  [Logo Senda] [Scope Switcher ▼] [...] [User Menu ▼]│
├──────────┬───────────────────────────────────────────┤
│ SIDEBAR  │  CONTENT AREA                             │
│          │                                           │
│ Dashboard│                                           │
│ Emails   │                                           │
│ Templates│                                           │
│ Injectors│                                           │
│ Adapters │                                           │
│ Domains  │                                           │
│ Webhooks │                                           │
│ Members  │                                           │
│ API Keys │                                           │
│ Audit Log│                                           │
│ Settings │                                           │
│          │                                           │
│ ──────── │                                           │
│ [?] Help │                                           │
└──────────┴───────────────────────────────────────────┘
```

**Mobile:** Sidebar collapsa a hamburger menu. Header sticky.

### 3.2. Scope Switcher

Elemento central de navegación. Muestra el scope activo y permite cambiar:

```
┌─────────────────────────────────────────┐
│  Scope Switcher                         │
│  ┌───────────────────────────────────┐  │
│  │ 🌐 Global                        │  │  ← Solo superadmin
│  ├───────────────────────────────────┤  │
│  │ 🏢 Tenant: LATAM                 │  │
│  │   ├── ⚙️ _system                 │  │
│  │   ├── 📦 acme-corp               │  │
│  │   └── 📦 brand-x                 │  │
│  ├───────────────────────────────────┤  │
│  │ 🏢 Tenant: Europe                │  │
│  │   └── 📦 uk-team                 │  │
│  └───────────────────────────────────┘  │
│  [+ Crear Tenant]  (si superadmin)      │
│  [+ Crear Workspace]  (si tenant_admin) │
└─────────────────────────────────────────┘
```

Solo muestra scopes donde el usuario tiene rol. Al seleccionar un scope, toda la UI se filtra a ese contexto.

### 3.3. Mapa de Pantallas

```
Onboarding (primer uso)
├── Welcome
├── Crear primer tenant
└── Crear primer workspace

Login (OIDC redirect)
├── Auth exitosa → Dashboard
└── No es miembro → Pantalla de acceso denegado

Dashboard (home)
├── Métricas de envío (scope actual)
├── Actividad reciente
└── Alertas (bounce rate, dominios con error)

Emails
├── Lista de emails (tabla paginada + filtros)
├── Detalle de email (timeline de eventos)
└── Búsqueda (por tracking_id, external_id, destinatario)

Templates
├── Template Types (lista + CRUD)
│   └── Asignar adapter
├── Templates por tipo (lista)
│   ├── Crear/editar template
│   ├── Versiones (lista con estados)
│   │   ├── Editor de versión (MJML visual + código)
│   │   ├── Preview (desktop + mobile)
│   │   ├── Locales (i18n por versión)
│   │   └── Publicar / Archivar
│   └── Desactivar template (kill switch)
└── Test send (preview + envío de prueba)

Injectors
├── Lista de definiciones (indica scope + herencia)
├── Crear/editar definición (schema: campos + tipos)
└── Editar valores (por scope, campo por campo)

Adapters
├── Lista de adapters (indica scope + default)
├── Crear/editar adapter (tipo + credentials)
└── Test connection

Domains
├── Lista de dominios (con status badge)
├── Registrar dominio (genera DNS records)
├── Ver DNS records (copiables)
└── Verificar dominio (manual + status)

Webhooks
├── Lista de webhooks
├── Crear/editar webhook (URL + eventos)
└── Test webhook (enviar ping)

Members
├── Lista de miembros (con roles por scope)
├── Invitar miembro (email + rol + scope)
└── Editar roles / Revocar acceso

API Keys
├── Lista de keys (hint visible, key oculta)
├── Generar nueva key (muestra key una sola vez)
└── Revocar key

Audit Log
├── Lista de eventos (tabla paginada + filtros)
└── Detalle de evento (cambios, antes/después)

Settings (solo scope global)
├── Configuración general
├── OIDC settings
└── Email defaults (retries, retention, rate limits)
```

---

## 4. Flujos de Usuario

### 4.1. Onboarding (Primer Uso)

**Trigger:** Primera vez que alguien accede a Senda con DB vacía.

```
[Usuario abre Senda] → [Pantalla Welcome]
    "Bienvenido a Senda. Configura tu plataforma en 3 pasos."

→ Paso 1: Login con OIDC
    [Botón: "Conectar con tu proveedor OIDC"]
    → Redirect a OIDC provider → Callback
    → Auto-registro como superadmin

→ Paso 2: Crear primer tenant
    - Campo: Código (slug, auto-suggest desde nombre)
    - Campo: Nombre
    [Botón: "Crear Tenant"]
    → Crea tenant + workspace _system automáticamente

→ Paso 3: Crear primer workspace
    - Campo: Código (slug)
    - Campo: Nombre
    [Botón: "Crear Workspace"]
    → Redirect al Dashboard

→ [Dashboard con guía contextual]
    Checklist visible:
    ☐ Configurar adapter de envío
    ☐ Verificar un dominio
    ☐ Crear primer template type
    ☐ Crear primer template
    ☐ Enviar primer email de prueba
```

**Estado:** El checklist persiste hasta completar todos los pasos. Es dismissible.

### 4.2. Envío desde API (no hay pantalla — referencia para contexto)

```
[Servicio externo] → POST /api/v1/send
    {
      to: "user@example.com",
      template_type: "latam:acme:welcome",
      variables: { user_name: "Juan" },
      locale: "es"
    }
→ Respuesta: { tracking_id: "abc123", status: "queued" }
```

El dashboard muestra el resultado en la sección Emails.

### 4.3. Crear y Publicar un Template

```
[Navegar a Templates] → [Ver Template Types]
→ Seleccionar tipo "welcome"
→ [Ver templates de ese tipo]
    - Si no hay template en scope actual:
      [Botón: "+ Crear Template"]
    - Si ya hay template:
      [Ver versiones]

→ [Crear nueva versión (draft)]
    → Abre editor con 2 modos:
      - Visual (drag-and-drop MJML blocks)
      - Código (editor MJML con syntax highlighting)
    → Panel lateral: subject, preview text, from_name, from_email, reply_to
    → Barra de variables: selector de variables del template type + injectors
    → Preview: toggle desktop/mobile
    → Locales: tab para agregar traducciones

→ [Guardar Draft]
    → Draft visible en lista de versiones

→ [Publicar] (solo admin+)
    → Confirma: "¿Publicar versión 3? La versión 2 se archivará."
    → Versión anterior → archived
    → Nueva versión → published
```

### 4.4. Configurar Adapter + Asignar a Template Type

```
[Navegar a Adapters] → [+ Crear Adapter]
    - Nombre: "SES Producción"
    - Tipo: SES | Gmail (selector)
    - Formulario dinámico según tipo:
      SES: Region, Access Key ID, Secret Access Key
      Gmail: OAuth Client ID, Client Secret, Refresh Token, Delegate Email
    - Rate limit: emails/segundo (default: 14)
    [Botón: "Guardar"]
    → Credentials se encriptan automáticamente
    → [Botón: "Test Connection"] → Envía email de prueba al propio usuario

→ [Navegar a Template Types] → Seleccionar tipo
    → Campo: "Adapter asignado" [Dropdown de adapters disponibles]
    → Guardar
```

### 4.5. Registrar y Verificar Dominio

```
[Navegar a Domains] → [+ Registrar Dominio]
    - Campo: dominio (ej: example.com)
    [Botón: "Registrar"]
    → Senda genera: DKIM key, DNS records

→ [Ver dominio registrado]
    Status: ⏳ Pendiente

    Sección "Registros DNS a configurar":
    ┌──────────────────────────────────────────────────────┐
    │ Tipo │ Nombre                    │ Valor        │ 📋 │
    │ TXT  │ senda._domainkey.example… │ v=DKIM1; ... │ 📋 │
    │ TXT  │ example.com               │ v=spf1 ...   │ 📋 │
    │ TXT  │ _dmarc.example.com        │ v=DMARC1; .. │ 📋 │
    └──────────────────────────────────────────────────────┘
    (Cada valor con botón copiar al clipboard)

    [Botón: "Verificar Ahora"]
    → Check DNS → Status: ✅ Verificado | ❌ Error (con detalle)
```

### 4.6. Gestionar Inyectores (Herencia Campo a Campo)

```
[Navegar a Injectors]
→ Lista muestra: definiciones del scope actual + heredadas
    Cada fila indica: nombre, scope origen, cantidad de campos

    ┌──────────────────────────────────────────────┐
    │ Nombre     │ Scope      │ Campos │ Acciones  │
    │ brand      │ 🌐 Global  │ 3      │ [Valores] │
    │ footer     │ ⚙️ _system │ 2      │ [Valores] │
    │ support    │ 📦 Actual  │ 4      │ [Editar]  │
    └──────────────────────────────────────────────┘

→ [Editar valores de "brand" para scope actual]
    Muestra campos del schema con valor actual por nivel:

    ┌──────────────────────────────────────────────────────┐
    │ Campo: logo                                         │
    │ Tipo: img                                           │
    │ Valor global:    corp-logo.png (🌐)                 │
    │ Valor _system:   — (hereda global)                  │
    │ Valor workspace: [_____________] [Guardar]          │
    │                  "Dejar vacío para heredar de arriba"│
    ├─────────────────────────────────────────────────────┤
    │ Campo: company_name                                 │
    │ Tipo: text                                          │
    │ Valor global:    "MiEmpresa" (🌐)                   │
    │ Valor _system:   "MiEmpresa LATAM" (⚙️)            │
    │ Valor workspace: [_____________] [Guardar]          │
    └──────────────────────────────────────────────────────┘
```

### 4.7. Monitorear Emails

```
[Navegar a Emails]
→ Tabla paginada con filtros:
    Filtros: status, fecha (rango), destinatario, template type, external_id
    Búsqueda: por tracking_id o email

    ┌───────────────────────────────────────────────────────────────┐
    │ To             │ Template    │ Status      │ Fecha            │
    │ user@test.com  │ welcome     │ ✅ delivered │ 2026-02-16 14:30│
    │ bob@corp.com   │ invoice     │ 📨 sent     │ 2026-02-16 14:28│
    │ bad@invalid.xx │ welcome     │ ❌ bounced  │ 2026-02-16 14:25│
    │ spam@user.com  │ notification│ ⚠️ complained│ 2026-02-16 14:20│
    └───────────────────────────────────────────────────────────────┘

→ [Click en email] → Detalle:
    Panel con info del email:
    - To, From, Subject, Template (link), Adapter usado
    - Tracking ID, External ID (si existe)
    - Variables snapshot, Injectors snapshot (collapsible)

    Timeline de eventos:
    ┌────────────────────────────────────────────┐
    │ ● Queued        │ 14:30:01                 │
    │ ● Sent          │ 14:30:03 (SES, msg-id)   │
    │ ● Delivered     │ 14:30:15                  │
    │ ● Opened        │ 14:35:22 (si tracking on) │
    └────────────────────────────────────────────┘
```

### 4.8. Gestionar Members y Roles

```
[Navegar a Members]
→ Lista de miembros con sus roles en el scope actual:

    ┌──────────────────────────────────────────────────────────┐
    │ Email              │ Nombre    │ Rol             │       │
    │ rey@empresa.com    │ Rey       │ superadmin (🌐)  │ [...]│
    │ maria@empresa.com  │ María     │ tenant_admin (🏢)│ [...]│
    │ dev@empresa.com    │ Dev Team  │ ws_editor (📦)   │ [...]│
    └──────────────────────────────────────────────────────────┘

→ [+ Invitar Miembro]
    - Campo: Email
    - Selector: Rol
    - Selector: Scope (según rol)
    [Botón: "Invitar"]
    → Registra en DB. Al hacer login OIDC tendrá acceso.
    (No se envía email de invitación en P1 — el admin avisa manualmente)
```

---

## 5. Pantallas Detalladas

### 5.1. Login / Access Denied

**Login:**
- Pantalla centrada, logo Senda
- Botón único: "Iniciar sesión con [Provider]"
- Redirect a OIDC → callback → dashboard

**Access Denied (usuario autenticado pero no miembro):**
- Pantalla centrada
- Mensaje: "Acceso denegado. Tu email (user@example.com) no está registrado como miembro. Contacta a tu administrador."
- Botón: "Cerrar sesión"

### 5.2. Dashboard (Home)

**Contenido según scope:**

**Scope Global (superadmin):**
- Métricas globales: total emails hoy, delivery rate, bounce rate, complaint rate
- Gráfico: emails enviados últimos 7/30 días (line chart)
- Top 5 tenants por volumen
- Alertas activas (dominios con error, bounce rate alto)
- Checklist de onboarding (si no completado)

**Scope Tenant:**
- Métricas del tenant: mismas métricas pero filtradas
- Top 5 workspaces del tenant por volumen
- Alertas del tenant

**Scope Workspace:**
- Métricas del workspace
- Últimos 10 emails enviados (mini-tabla)
- Templates con drafts pendientes de publicar
- Alertas del workspace

### 5.3. Emails — Lista

- **Tabla responsive:** En mobile, colapsa a cards
- **Columnas:** Destinatario, Template Type, Status (badge con color), Fecha, Tracking ID
- **Filtros (panel colapsable en mobile):**
  - Status: multiselect (queued, sent, delivered, bounced, complained, failed, suppressed)
  - Fecha: date range picker
  - Template Type: dropdown
  - Búsqueda: text input (busca en tracking_id, external_id, destinatario, remitente)
- **Paginación:** Cursor-based (botones "Anterior" / "Siguiente", no page numbers)
- **Empty state:** "No hay emails en este período. Los emails aparecerán aquí cuando se envíen vía API."

### 5.4. Emails — Detalle

- **Header:** Status badge grande + destinatario + subject
- **Sección Info:**
  - To, CC, BCC
  - From (email + name)
  - Template: link al template type + versión usada
  - Adapter: nombre del adapter usado
  - Tracking ID (copiable)
  - External ID (copiable, si existe)
  - Locale
  - Fecha de envío
- **Sección Timeline:** Vertical, iconos por tipo de evento, timestamp
- **Sección Snapshots (collapsible):**
  - Variables enviadas (JSON formateado)
  - Injectors resueltos (JSON formateado)
- **Sección Error (si failed/bounced):**
  - Mensaje de error del provider
  - Bounce type (soft/hard)
  - Retry count

### 5.5. Template Types — Lista

- **Tabla:** Slug, Nombre, Adapter asignado (badge o "⚠️ Sin adapter"), Scope origen, Templates count
- **Acciones por fila:** Editar, Asignar adapter
- **Empty state:** "No hay tipos de plantilla configurados. Crea un tipo para definir el contrato de variables que tus templates usarán."
- **Indicador de herencia:** Icon que muestra si viene de global, _system, o es propio

### 5.6. Templates — Lista y Versiones

**Lista de templates por tipo:**
- Muestra si hay template en scope actual, en _system, o global
- Si hay template propio: link al editor
- Si no hay: botón "Crear template para este scope"
- Si hereda: badge "Heredado de [scope]" con opción de crear override

**Versiones de un template:**
- Tabla: Versión #, Status (badge: draft/published/archived), Creado por, Fecha, Acciones
- Solo una versión puede ser "published" (highlighted)
- Acciones: Editar (si draft), Publicar (si draft + admin), Archivar, Preview

### 5.7. Editor de Template

**Layout de 2 paneles (o tabs en mobile):**

```
┌─────────────────────────┬──────────────────────────┐
│ EDITOR                  │ PREVIEW                  │
│                         │                          │
│ [Visual] [Código]       │ [Desktop] [Mobile]       │
│                         │                          │
│ ┌─────────────────────┐ │ ┌──────────────────────┐ │
│ │ Drag-and-drop MJML  │ │ │ Rendered HTML        │ │
│ │ blocks:             │ │ │ preview              │ │
│ │                     │ │ │                      │ │
│ │ [Header]            │ │ │                      │ │
│ │ [Text Block]        │ │ │                      │ │
│ │ [Image]             │ │ │                      │ │
│ │ [Button]            │ │ │                      │ │
│ │ [Divider]           │ │ │                      │ │
│ │ [Columns]           │ │ │                      │ │
│ │                     │ │ │                      │ │
│ └─────────────────────┘ │ └──────────────────────┘ │
│                         │                          │
│ ── Metadatos ────────── │                          │
│ Subject: [___________]  │                          │
│ Preview: [___________]  │                          │
│ From:    [___________]  │                          │
│ Reply-to:[___________]  │                          │
│                         │                          │
│ ── Variables ────────── │                          │
│ {{user_name}}  [Insert] │                          │
│ {{brand.logo}} [Insert] │                          │
│                         │                          │
│ ── Locales ──────────── │                          │
│ [es] [en] [pt] [+ Add] │                          │
└─────────────────────────┴──────────────────────────┘

[Guardar Draft]  [Preview Completo]  [Enviar Test]  [Publicar ▼]
```

**Notas para UX/UI:**
- El editor visual es un MJML block editor (similar a Stripo, Unlayer)
- Modo código: syntax highlighting MJML con autocompletado de variables
- Preview se actualiza en tiempo real al editar
- Variables disponibles se muestran en sidebar/panel con click-to-insert
- Locales: tabs para editar subject + body per locale. Base = default_locale
- "Enviar Test": abre modal para ingresar email destinatario + variables de prueba
- "Publicar": solo visible para admin+. Requiere confirmación.

### 5.8. Injectors — Editar Valores

**Layout para edición campo por campo:**

Cada campo del injector se muestra con su cadena de herencia visual:

```
Campo: logo (tipo: img)
┌──────────────────────────────────────────────┐
│ 🌐 Global:     corp-logo.png                │
│ ⚙️ _system:    — (hereda global)            │
│ 📦 Workspace:  [___________] [Guardar] [🗑️] │
│                                              │
│ Valor efectivo: corp-logo.png (de Global)    │
└──────────────────────────────────────────────┘
```

- Input type cambia según field_type: text → text input, img → URL + preview, html → rich text, url → URL input, number → number input, bool → toggle
- Al guardar un override, el "valor efectivo" cambia
- Al borrar override (🗑️), vuelve a heredar
- Visual claro de qué nivel provee el valor actual

### 5.9. Adapters — CRUD

**Lista:**
- Nombre, Tipo (badge: SES/Gmail), Scope, Default (✓ o —), Rate limit, Status
- Indicador: "Usado por N template types"

**Crear/Editar:**
- Nombre
- Tipo: selector (SES, Gmail) — cambia el formulario de credentials dinámicamente
- Credentials (formulario según tipo):
  - SES: Region, Access Key ID, Secret Access Key
  - Gmail: OAuth Client ID, Client Secret, Refresh Token, Delegate Email
- Rate limit: number input (emails/seg), default 14
- Marcar como default: toggle
- [Test Connection]: envía email de prueba al email del admin actual

**Seguridad visual:** Los campos de credentials muestran "••••••••" después de guardar. No se pueden ver otra vez, solo reemplazar.

### 5.10. Domains

**Lista:**
- Dominio, Status badge (⏳ Pending / ✅ Verified / ❌ Error), Scope, Última verificación
- Click → detalle

**Detalle:**
- Status grande con última verificación
- Tabla de DNS records copiables (botón copy por fila)
- Si error: mensaje de error específico ("TXT record not found", "DKIM key mismatch")
- [Verificar Ahora] → loading → resultado
- Timeline de verificaciones (últimas 5)

### 5.11. Webhooks

**Lista:**
- URL (truncada), Eventos suscritos (badges), Status (active/disabled), Failures
- Si consecutive_failures > 0: warning badge

**Crear/Editar:**
- URL (HTTPS obligatorio)
- Eventos: multiselect checkboxes (sent, delivered, bounced, complained, failed)
- Auto-generado: Secret (mostrado una sola vez al crear)
- [Test Webhook]: envía ping → muestra respuesta (status code + latencia)

### 5.12. API Keys

**Lista:**
- Nombre, Hint (últimos 8 chars), Creado por, Fecha, Último uso, Status
- Nunca muestra el key completo

**Generar nueva:**
- Campo: Nombre (label descriptivo)
- [Generar]
- Modal: "Tu API Key ha sido generada. Cópiala ahora — no se mostrará otra vez."
  ```
  senda_live_a3f8b9c2d4e5f6a7b8c9d0e1f2a3b4c5
  ```
  [Copiar] [Cerrar]

**Revocar:**
- Confirmación: "¿Revocar key '{nombre}' (hint: ...b4c5)? Los servicios que la usan dejarán de funcionar."

### 5.13. Audit Log

**Tabla paginada con filtros:**
- Columnas: Fecha, Quién (email), Acción (badge), Recurso, Scope
- Filtros: fecha (rango), acción (multiselect), recurso type, miembro
- Click → detalle con cambios (JSON diff antes/después)

### 5.14. Settings (Global)

Solo visible para superadmin en scope global:
- **OIDC:** Discovery URL, Client ID (client secret oculto)
- **Email defaults:** Max retries, backoff base, log retention days
- **Alertas:** Bounce threshold %, Complaint threshold %
- **Dominio:** Recheck interval hours

---

## 6. Patrones de UI y Componentes

### 6.1. Status Badges

| Status | Color | Icon |
|--------|-------|------|
| queued | gray | ⏳ |
| processing | blue | ⏳ |
| sent | blue | 📨 |
| delivered | green | ✅ |
| opened | green | 👁️ |
| bounced | red | ❌ |
| complained | orange | ⚠️ |
| failed | red | ❌ |
| suppressed | gray | 🚫 |
| draft | gray | 📝 |
| published | green | ✅ |
| archived | gray | 📦 |
| pending | yellow | ⏳ |
| verified | green | ✅ |
| error | red | ❌ |
| active | green | ● |
| disabled | gray | ● |
| revoked | red | ● |

### 6.2. Scope Indicators

Siempre indicar de dónde viene un recurso heredado:
- 🌐 = Global
- ⚙️ = _system (tenant)
- 📦 = Workspace actual

### 6.3. Herencia Visual

Cuando un recurso es heredado, mostrarlo con:
- Badge de scope origen
- Texto sutil: "Heredado de [scope]"
- Si está soft-deleted: warning amarillo "Este recurso está marcado como deprecado en [scope]. Se recomienda crear un override."
- Acción visible: "Crear override" para reemplazar en el scope actual

### 6.4. Tables

- Header sticky
- Row hover highlight
- Cursor pagination (no page numbers): [← Anterior] [Siguiente →]
- Responsive: en mobile colapsan a card layout
- Empty state siempre con mensaje descriptivo + acción sugerida
- Loading: skeleton rows

### 6.5. Forms

- Validación inline (no esperar submit)
- Errores bajo el campo con texto rojo
- Campos requeridos: asterisco rojo
- Slugs: auto-generate desde nombre (con opción de editar)
- Credentials: password fields con toggle show/hide

### 6.6. Confirmaciones Destructivas

Para acciones con consecuencias (publicar, revocar, eliminar, purge):
- Modal de confirmación
- Texto claro del impacto
- Para purge: mostrar lista de dependencias afectadas
- Input de confirmación para acciones críticas: "Escribe DELETE para confirmar"

### 6.7. Toasts / Notifications

- Éxito: toast verde (auto-dismiss 3s)
- Error: toast rojo (persiste hasta dismiss)
- Warning: toast amarillo (auto-dismiss 5s)
- Posición: bottom-right (desktop), bottom-center (mobile)

### 6.8. Loading States

- Tablas: skeleton rows
- Formularios: botón disabled + spinner
- Páginas: spinner centrado
- Acciones inline: spinner reemplaza el botón

---

## 7. Responsive Breakpoints

| Breakpoint | Nombre | Layout |
|-----------|--------|--------|
| < 640px | Mobile | Single column, hamburger nav, cards instead of tables |
| 640–1024px | Tablet | Sidebar colapsable, tables with horizontal scroll |
| > 1024px | Desktop | Sidebar visible, full tables, 2-panel editor |

**Mobile-first:** Diseñar primero la experiencia mobile, luego expandir.

---

## 8. Empty States

Cada pantalla sin datos debe tener un empty state útil:

| Pantalla | Mensaje | Acción |
|----------|---------|--------|
| Dashboard (nuevo) | "Bienvenido a Senda. Completa la configuración inicial." | Link a checklist |
| Emails | "No hay emails enviados aún. Configura un template y envía tu primer email vía API." | Link a docs de API |
| Templates | "No hay template types configurados. Crea un tipo para definir el contrato de tus emails." | [+ Crear Template Type] |
| Injectors | "No hay injectors en este scope. Los injectors proveen datos automáticos a tus templates." | [+ Crear Injector] |
| Adapters | "No hay adapters configurados. Necesitas al menos un adapter para enviar emails." | [+ Crear Adapter] |
| Domains | "No hay dominios verificados. Registra un dominio para poder enviar emails." | [+ Registrar Dominio] |
| Webhooks | "No hay webhooks configurados. Los webhooks notifican a tus servicios sobre eventos de email." | [+ Crear Webhook] |
| API Keys | "No hay API Keys activas. Genera una key para que tus servicios envíen emails." | [+ Generar API Key] |
| Audit Log | "No hay eventos registrados aún." | — |

---

## 9. Consideraciones de Accesibilidad

- Contraste WCAG 2.1 AA mínimo
- Todos los formularios con labels asociados (no solo placeholder)
- Focus visible en todos los elementos interactivos
- Navegación por teclado completa (tab order)
- Screen reader: ARIA labels en badges, iconos, y scope indicators
- Color no es el único indicador de status (siempre acompañar con icono o texto)

---

## 10. Editor Visual de Templates (Especificación para Fase 3)

El editor visual de templates es la funcionalidad más compleja del dashboard. En P1/Fase 1, puede ser solo editor de código MJML con preview. El editor drag-and-drop completo viene en Fase 3.

**Fase 1 (P1) — Editor mínimo:**
- Textarea con syntax highlighting para MJML
- Preview panel (HTML renderizado desde MJML)
- Panel de variables disponibles (click to insert)
- Campos de metadatos (subject, from, etc.)
- Tabs de locales

**Fase 3 — Editor visual completo:**
- Drag-and-drop de bloques MJML
- Inline editing de texto sobre el preview
- Selector visual de variables (dropdown en posición del cursor)
- AI assistance para traducciones y copy
- Undo/redo
- Version diff (comparar dos versiones)

---

## 11. Entregables Esperados del UX/UI

1. **Wireframes** de todas las pantallas listadas en §3.3 (mobile + desktop)
2. **Prototipos navegables** de los flujos principales (§4.1 a §4.8)
3. **Design system** basado en Tailwind + shadcn/ui:
   - Palette de colores (primary, status colors, scope colors)
   - Tipografía (headings, body, code)
   - Componentes customizados: scope switcher, herencia visual, status badges
4. **Especificación de componentes** para: scope switcher, inheritance indicator, template editor (fase 1), timeline de eventos
5. **Responsive specs** para breakpoints definidos en §7
