import dataPlaneOpenApi from "@/lib/data-plane-openapi.json";

type OpenAPIParameter = {
  name: string;
  in: "path" | "query" | "header" | string;
  required?: boolean;
  description?: string;
  schema?: OpenAPISchema;
};

type OpenAPIContent = {
  schema?: OpenAPISchema;
  example?: unknown;
};

type OpenAPIResponse = {
  description?: string;
  content?: Record<string, OpenAPIContent>;
};

type OpenAPIOperation = {
  summary?: string;
  description?: string;
  security?: Array<Record<string, string[]>>;
  parameters?: OpenAPIParameter[];
  requestBody?: {
    required?: boolean;
    content?: Record<string, OpenAPIContent>;
  };
  responses?: Record<string, OpenAPIResponse>;
};

type OpenAPIPathItem = Record<string, OpenAPIOperation>;

type OpenAPISchema = {
  $ref?: string;
  type?: string;
  format?: string;
  description?: string;
  enum?: string[];
  default?: unknown;
  required?: string[];
  properties?: Record<string, OpenAPISchema>;
  items?: OpenAPISchema;
  allOf?: OpenAPISchema[];
  additionalProperties?: boolean | OpenAPISchema;
  minItems?: number;
  maxItems?: number;
};

type FieldRow = {
  name: string;
  type: string;
  required: boolean;
  description?: string;
};

type OpenAPIDocument = {
  paths: Record<string, OpenAPIPathItem>;
  components?: {
    schemas?: Record<string, OpenAPISchema>;
    securitySchemes?: Record<
      string,
      {
        description?: string;
        type?: string;
        scheme?: string;
        bearerFormat?: string;
      }
    >;
  };
};

const document = dataPlaneOpenApi as OpenAPIDocument;
const schemas = document.components?.schemas ?? {};
const securitySchemes = document.components?.securitySchemes ?? {};

const endpointEntries = Object.entries(document.paths).flatMap(([path, pathItem]) =>
  Object.entries(pathItem).map(([method, operation]) => ({
    id: `${method}-${path}`,
    method: method.toUpperCase(),
    path,
    operation,
  })),
);

export function ApiEndpointReference() {
  return (
    <div className="space-y-4">
      {endpointEntries.map((endpoint) => {
        const authenticationLines = getAuthenticationLines(endpoint.operation);
        const headerRows = getHeaderRows(endpoint.operation);
        const pathParameters = getParametersByLocation(endpoint.operation, "path");
        const queryParameters = getParametersByLocation(endpoint.operation, "query");
        const requestBodyRows = getRequestBodyRows(endpoint.operation);
        const requestExample = getRequestExample(endpoint.operation);
        const responses = Object.entries(endpoint.operation.responses ?? {});

        return (
          <details
            key={endpoint.id}
            className="group overflow-hidden rounded-xl border border-border/80 bg-card/50"
          >
            <summary className="flex cursor-pointer list-none items-start gap-4 px-5 py-4 marker:content-none">
              <MethodBadge method={endpoint.method} />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-3">
                  <code className="rounded bg-muted px-2 py-1 text-sm font-semibold text-foreground">
                    {endpoint.path}
                  </code>
                </div>
                <p className="mt-2 text-sm text-muted-foreground">
                  {endpoint.operation.summary ?? endpoint.operation.description ?? "Endpoint reference"}
                </p>
              </div>
              <span className="mt-0.5 text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground transition-transform group-open:rotate-90">
                ›
              </span>
            </summary>

            <div className="space-y-6 border-t border-border/70 px-5 py-5">
              <Section title="Authentication">
                <SimpleList items={authenticationLines} emptyLabel="None" />
              </Section>

              <Section title="Headers">
                <FieldTable rows={headerRows} emptyLabel="None" />
              </Section>

              <Section title="Path parameters">
                <FieldTable rows={pathParameters} emptyLabel="None" />
              </Section>

              <Section title="Query parameters">
                <FieldTable rows={queryParameters} emptyLabel="None" />
              </Section>

              <Section title="Request body">
                <div className="space-y-4">
                  <FieldTable rows={requestBodyRows} emptyLabel="None" />
                  {requestExample ? (
                    <CodeBlock value={JSON.stringify(requestExample, null, 2)} />
                  ) : (
                    <EmptyLabel label="None" />
                  )}
                </div>
              </Section>

              <Section title="Responses">
                {responses.length > 0 ? (
                  <div className="space-y-4">
                    {responses.map(([statusCode, response]) => (
                      <ResponseCard
                        key={statusCode}
                        statusCode={statusCode}
                        response={response}
                      />
                    ))}
                  </div>
                ) : (
                  <EmptyLabel label="None" />
                )}
              </Section>
            </div>
          </details>
        );
      })}
    </div>
  );
}

function MethodBadge({ method }: { method: string }) {
  const tone =
    method === "POST"
      ? "bg-emerald-500/15 text-emerald-300 ring-emerald-400/30"
      : "bg-sky-500/15 text-sky-300 ring-sky-400/30";

  return (
    <span
      className={`inline-flex min-w-16 justify-center rounded-full px-2.5 py-1 text-xs font-semibold uppercase tracking-[0.18em] ring-1 ${tone}`}
    >
      {method}
    </span>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div>
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      </div>
      {children}
    </section>
  );
}

function SimpleList({
  items,
  emptyLabel,
}: {
  items: string[];
  emptyLabel: string;
}) {
  if (items.length === 0) return <EmptyLabel label={emptyLabel} />;

  return (
    <ul className="ml-0 space-y-2 pl-0">
      {items.map((item) => (
        <li key={item} className="ml-0 list-none rounded-lg border border-border/70 bg-background/40 px-3 py-2 text-sm text-muted-foreground">
          {item}
        </li>
      ))}
    </ul>
  );
}

function FieldTable({
  rows,
  emptyLabel,
}: {
  rows: FieldRow[];
  emptyLabel: string;
}) {
  if (rows.length === 0) return <EmptyLabel label={emptyLabel} />;

  return (
    <div className="overflow-hidden rounded-lg border border-border/70">
      <div className="grid grid-cols-[minmax(0,1.3fr)_minmax(0,0.9fr)_auto] gap-3 border-b border-border/70 bg-muted/40 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        <span>Name</span>
        <span>Type</span>
        <span>Required</span>
      </div>
      {rows.map((row) => (
        <div
          key={`${row.name}-${row.type}`}
          className="grid grid-cols-[minmax(0,1.3fr)_minmax(0,0.9fr)_auto] gap-3 border-b border-border/50 px-4 py-3 last:border-b-0"
        >
          <div className="min-w-0">
            <div className="font-mono text-sm text-foreground">{row.name}</div>
            {row.description ? (
              <p className="mt-1 text-sm leading-6 text-muted-foreground">
                {row.description}
              </p>
            ) : null}
          </div>
          <div className="text-sm text-muted-foreground">{row.type}</div>
          <div className="text-sm text-muted-foreground">
            {row.required ? "Yes" : "No"}
          </div>
        </div>
      ))}
    </div>
  );
}

function ResponseCard({
  statusCode,
  response,
}: {
  statusCode: string;
  response: OpenAPIResponse;
}) {
  const contentEntries = Object.entries(response.content ?? {});

  return (
    <details className="group/status-response overflow-hidden rounded-lg border border-border/70 bg-background/40">
      <summary className="flex cursor-pointer list-none flex-wrap items-center justify-between gap-3 px-4 py-3 marker:content-none">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <span className="rounded-full bg-muted px-2.5 py-1 text-xs font-semibold text-foreground">
            {statusCode}
          </span>
          <span className="text-sm text-muted-foreground">
            {response.description ?? "Response"}
          </span>
        </div>
        <div className="flex items-center gap-3 text-xs uppercase tracking-[0.16em] text-muted-foreground">
          <span className="group-open/status-response:hidden">View response details</span>
          <span className="hidden group-open/status-response:inline">Hide response details</span>
          <span className="transition-transform group-open/status-response:rotate-90">›</span>
        </div>
      </summary>
      <div className="space-y-4 border-t border-border/70 px-4 py-4">
        {contentEntries.length > 0 ? (
          contentEntries.map(([contentType, content]) => {
            const summaryRows = getSchemaSummaryRows(content.schema);
            const detailRows = getSchemaRows(content.schema);
            const example = content.example ?? getExampleValue(content.schema, contentType);
            return (
              <div key={contentType} className="space-y-3">
                <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                  <span className="font-semibold text-foreground">Content-Type:</span>{" "}
                  {contentType}
                </div>
                <div className="space-y-2">
                  <h4 className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                    Schema summary
                  </h4>
                  <FieldTable rows={summaryRows} emptyLabel="None" />
                </div>
                <div className="space-y-2">
                  <h4 className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                    Example
                  </h4>
                  {example !== null ? (
                    <CodeBlock value={formatExampleValue(example, contentType)} />
                  ) : (
                    <EmptyLabel label="None" />
                  )}
                </div>
                <details className="group/response rounded-md border border-border/70 bg-background/20">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-3 py-2 text-sm text-muted-foreground marker:content-none">
                    <span className="group-open/response:hidden">View full schema fields</span>
                    <span className="hidden group-open/response:inline">Hide full schema fields</span>
                    <span className="text-xs uppercase tracking-[0.16em] transition-transform group-open/response:rotate-90">
                      ›
                    </span>
                  </summary>
                  <div className="border-t border-border/70 p-3">
                    <FieldTable rows={detailRows} emptyLabel="None" />
                  </div>
                </details>
              </div>
            );
          })
        ) : (
          <EmptyLabel label="None" />
        )}
      </div>
    </details>
  );
}

function CodeBlock({ value }: { value: string }) {
  return (
    <pre className="overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-4 text-xs text-slate-50">
      <code>{value}</code>
    </pre>
  );
}

function EmptyLabel({ label }: { label: string }) {
  return (
    <div className="rounded-lg border border-dashed border-border/70 bg-background/30 px-3 py-2 text-sm text-muted-foreground">
      {label}
    </div>
  );
}

function getAuthenticationLines(operation: OpenAPIOperation): string[] {
  const lines = new Set<string>();

  for (const security of operation.security ?? []) {
    for (const schemeName of Object.keys(security)) {
      const scheme = securitySchemes[schemeName];
      if (schemeName === "WorkspaceAPIKeyBearer") {
        lines.add("Workspace API key via Authorization: Bearer <api-key>");
      } else if (scheme?.description) {
        lines.add(scheme.description);
      } else {
        lines.add(schemeName);
      }
    }
  }

  return Array.from(lines);
}

function getHeaderRows(operation: OpenAPIOperation): FieldRow[] {
  const rows: FieldRow[] = [];

  if ((operation.security ?? []).some((security) => "WorkspaceAPIKeyBearer" in security)) {
    rows.push({
      name: "Authorization",
      type: "Bearer token",
      required: true,
      description: "Use the workspace API key as `Bearer <api-key>`.",
    });
  }

  for (const contentType of Object.keys(operation.requestBody?.content ?? {})) {
    rows.push({
      name: "Content-Type",
      type: contentType,
      required: true,
      description: `Request body content type for this endpoint.`,
    });
  }

  return rows;
}

function getParametersByLocation(
  operation: OpenAPIOperation,
  location: "path" | "query",
): FieldRow[] {
  return (operation.parameters ?? [])
    .filter((parameter) => parameter.in === location)
    .map((parameter) => ({
      name: parameter.name,
      type: formatSchemaType(parameter.schema),
      required: Boolean(parameter.required),
      description: formatSchemaDescription(parameter.schema, parameter.description),
    }));
}

function getRequestBodyRows(operation: OpenAPIOperation): FieldRow[] {
  const firstContent = Object.values(operation.requestBody?.content ?? {})[0];
  const schemaRows = getSchemaRows(firstContent?.schema);

  if (schemaRows.length === 0 && operation.requestBody) {
    return [
      {
        name: "body",
        type: "object",
        required: Boolean(operation.requestBody.required),
        description: "Request body",
      },
    ];
  }

  return schemaRows;
}

function getRequestExample(operation: OpenAPIOperation): unknown | null {
  const firstContent = Object.values(operation.requestBody?.content ?? {})[0];
  return firstContent?.example ?? null;
}

function getSchemaRows(schema?: OpenAPISchema): FieldRow[] {
  if (!schema) return [];

  const resolved = resolveSchema(schema);

  if (!resolved.properties || Object.keys(resolved.properties).length === 0) {
    return [
      {
        name: "schema",
        type: formatSchemaType(resolved),
        required: true,
        description: formatSchemaDescription(resolved),
      },
    ];
  }

  const rows: FieldRow[] = [];
  collectSchemaRows(rows, resolved, "", new Set(resolved.required ?? []));
  return rows;
}

function getSchemaSummaryRows(schema?: OpenAPISchema): FieldRow[] {
  if (!schema) return [];

  const resolved = resolveSchema(schema);

  if (!resolved.properties || Object.keys(resolved.properties).length === 0) {
    return [
      {
        name: "schema",
        type: formatSchemaType(resolved),
        required: true,
        description: formatSchemaDescription(resolved),
      },
    ];
  }

  return Object.entries(resolved.properties).map(([name, childSchema]) => {
    const childResolved = resolveSchema(childSchema);
    const nestedKeys = getNestedFieldHint(childResolved);
    const description = formatSchemaDescription(childResolved);

    return {
      name,
      type: formatSchemaType(childResolved),
      required: new Set(resolved.required ?? []).has(name),
      description: [description, nestedKeys].filter(Boolean).join(" · ") || undefined,
    };
  });
}

function collectSchemaRows(
  rows: FieldRow[],
  schema: OpenAPISchema,
  prefix: string,
  requiredNames: Set<string>,
) {
  const resolved = resolveSchema(schema);

  for (const [name, childSchema] of Object.entries(resolved.properties ?? {})) {
    const childResolved = resolveSchema(childSchema);
    const fieldName = prefix ? `${prefix}.${name}` : name;

    rows.push({
      name: fieldName,
      type: formatSchemaType(childResolved),
      required: requiredNames.has(name),
      description: formatSchemaDescription(childResolved),
    });

    if (childResolved.type === "object" && childResolved.properties) {
      collectSchemaRows(
        rows,
        childResolved,
        fieldName,
        new Set(childResolved.required ?? []),
      );
    } else if (childResolved.type === "array" && childResolved.items) {
      const itemSchema = resolveSchema(childResolved.items);
      if (itemSchema.type === "object" && itemSchema.properties) {
        collectSchemaRows(
          rows,
          itemSchema,
          `${fieldName}[]`,
          new Set(itemSchema.required ?? []),
        );
      }
    }
  }
}

function resolveSchema(schema?: OpenAPISchema): OpenAPISchema {
  if (!schema) return {};

  if (schema.$ref) {
    const refName = schema.$ref.split("/").pop();
    if (!refName) return schema;
    return resolveSchema(schemas[refName]);
  }

  if (schema.allOf && schema.allOf.length > 0) {
    return schema.allOf.reduce<OpenAPISchema>(
      (merged, item) => mergeSchemas(merged, resolveSchema(item)),
      {},
    );
  }

  return schema;
}

function mergeSchemas(base: OpenAPISchema, extra: OpenAPISchema): OpenAPISchema {
  return {
    ...base,
    ...extra,
    required: Array.from(new Set([...(base.required ?? []), ...(extra.required ?? [])])),
    properties: {
      ...(base.properties ?? {}),
      ...(extra.properties ?? {}),
    },
  };
}

function formatSchemaType(schema?: OpenAPISchema): string {
  const resolved = resolveSchema(schema);

  if (resolved.type === "array") {
    return `array<${formatSchemaType(resolved.items)}>`;
  }

  if (resolved.enum && resolved.enum.length > 0) {
    return resolved.enum.map((value) => `"${value}"`).join(" | ");
  }

  if (resolved.type === "object" && resolved.additionalProperties) {
    if (resolved.additionalProperties === true) return "object";
    return `object<${formatSchemaType(resolved.additionalProperties)}>`;
  }

  return resolved.format ? `${resolved.type ?? "unknown"} (${resolved.format})` : resolved.type ?? "unknown";
}

function formatSchemaDescription(
  schema?: OpenAPISchema,
  fallback?: string,
): string | undefined {
  const resolved = resolveSchema(schema);
  const parts = [resolved.description ?? fallback].filter(Boolean) as string[];

  if (resolved.default !== undefined) {
    parts.push(`Default: ${String(resolved.default)}`);
  }
  if (resolved.minItems !== undefined || resolved.maxItems !== undefined) {
    const min = resolved.minItems !== undefined ? `min ${resolved.minItems}` : null;
    const max = resolved.maxItems !== undefined ? `max ${resolved.maxItems}` : null;
    parts.push([min, max].filter(Boolean).join(", "));
  }

  return parts.length > 0 ? parts.join(" · ") : undefined;
}

function getNestedFieldHint(schema?: OpenAPISchema): string | undefined {
  const resolved = resolveSchema(schema);

  if (resolved.type === "object" && resolved.properties) {
    const nestedFields = Object.keys(resolved.properties);
    if (nestedFields.length === 0) return undefined;
    return `Fields: ${nestedFields.join(", ")}`;
  }

  if (resolved.type === "array" && resolved.items) {
    const itemSchema = resolveSchema(resolved.items);

    if (itemSchema.type === "object" && itemSchema.properties) {
      const nestedFields = Object.keys(itemSchema.properties);
      if (nestedFields.length === 0) return undefined;
      return `Item fields: ${nestedFields.join(", ")}`;
    }
  }

  return undefined;
}

function getExampleValue(
  schema?: OpenAPISchema,
  contentType?: string,
): unknown | null {
  if (contentType === "text/csv") {
    return "tracking_id,status\ntr_01HXYZABC123,queued";
  }

  const resolved = resolveSchema(schema);

  if (resolved.default !== undefined) return resolved.default;
  if (resolved.enum && resolved.enum.length > 0) return resolved.enum[0];

  switch (resolved.type) {
    case "string":
      return getStringExample(resolved.format);
    case "integer":
      return 1;
    case "number":
      return 1;
    case "boolean":
      return true;
    case "array": {
      const itemExample = getExampleValue(resolved.items);
      return itemExample === null ? [] : [itemExample];
    }
    case "object": {
      const properties = resolved.properties ?? {};
      if (Object.keys(properties).length === 0) {
        if (resolved.additionalProperties && resolved.additionalProperties !== true) {
          const valueExample = getExampleValue(resolved.additionalProperties);
          return valueExample === null ? {} : { key: valueExample };
        }
        return {};
      }

      return Object.fromEntries(
        Object.entries(properties).map(([name, childSchema]) => [
          name,
          getExampleValue(childSchema),
        ]),
      );
    }
    default:
      return null;
  }
}

function getStringExample(format?: string): string {
  switch (format) {
    case "uuid":
      return "0195f8b7-8d8a-7c14-b0b8-5f0f7d6f4c21";
    case "email":
      return "hello@example.com";
    case "date-time":
      return "2026-04-03T12:00:00Z";
    case "date":
      return "2026-04-03";
    case "uri":
      return "https://api.example.com/resource";
    default:
      return "string";
  }
}

function formatExampleValue(value: unknown, contentType?: string): string {
  if (typeof value === "string" && contentType === "text/csv") {
    return value;
  }

  return JSON.stringify(value, null, 2);
}
