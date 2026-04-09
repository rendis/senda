import test from "node:test";
import assert from "node:assert/strict";

const {
  extractInjectorNamesFromTemplateMarkup,
  extractInjectorUsageFromTemplateMarkup,
  extractInjectorUsageFromTemplateSources,
  filterInjectorDefinitionsByTemplateUsage,
  resolveTestSendInjectorNames,
  resolveTestSendInjectorUsage,
} = await import(new URL("./test-send-injector-usage.ts", import.meta.url).href);

const injectorCatalog = [
  {
    name: "student",
    fields: [
      { field_name: "name" },
      { field_name: "last_name" },
      { field_name: "age" },
    ],
  },
  {
    name: "brand",
    fields: [
      { field_name: "logo_url" },
      { field_name: "support_email" },
    ],
  },
  {
    name: "unused",
    fields: [{ field_name: "field" }],
  },
];

test("extracts only injector names referenced by template placeholders", () => {
  const names = extractInjectorNamesFromTemplateMarkup(`
    <mj-text>Hello {{ injector.student.name }}</mj-text>
    <mj-text>{{ injector.brand.logo_url }}</mj-text>
    <mj-text>{{ event.user_name }}</mj-text>
    <mj-text>{{ injector.student.last_name }}</mj-text>
  `);

  assert.deepEqual(names, ["student", "brand"]);
});

test("extracts injector field usage from template placeholders", () => {
  const usage = extractInjectorUsageFromTemplateMarkup(`
    <mj-text>Hello {{ injector.student.name }}</mj-text>
    <mj-text>{{ injector.brand.logo_url }}</mj-text>
    <mj-text>{{ injector.student.last_name }}</mj-text>
  `);

  assert.deepEqual(usage, {
    student: ["name", "last_name"],
    brand: ["logo_url"],
  });
});

test("extracts injector fields whose names contain spaces", () => {
  const usage = extractInjectorUsageFromTemplateMarkup(`
    <mj-text>Hello {{ injector.student.last name }}</mj-text>
    <mj-text>{{ injector.student.favorite color }}</mj-text>
  `);

  assert.deepEqual(usage, {
    student: ["last name", "favorite color"],
  });
});

test("filters the injector catalog down to the fields used by the template", () => {
  const filtered = filterInjectorDefinitionsByTemplateUsage(injectorCatalog, {
    student: ["name"],
    brand: ["logo_url"],
  });

  assert.deepEqual(
    filtered.map((item: { name: string; fields: Array<{ field_name: string }> }) => ({
      name: item.name,
      fields: item.fields.map((field) => field.field_name),
    })),
    [
      { name: "student", fields: ["name"] },
      { name: "brand", fields: ["logo_url"] },
    ],
  );
});

test("filters the injector catalog when referenced field names contain spaces", () => {
  const filtered = filterInjectorDefinitionsByTemplateUsage(
    [
      {
        name: "student",
        fields: [
          { field_name: "name" },
          { field_name: "last name" },
          { field_name: "favorite color" },
        ],
      },
    ],
    {
      student: ["last name"],
    },
  );

  assert.deepEqual(
    filtered.map((item: { name: string; fields: Array<{ field_name: string }> }) => ({
      name: item.name,
      fields: item.fields.map((field) => field.field_name),
    })),
    [{ name: "student", fields: ["last name"] }],
  );
});

test("returns an empty injector list when the template does not reference injector variables", () => {
  const usage = extractInjectorUsageFromTemplateMarkup(`<mj-text>Hello {{ event.user_name }}</mj-text>`);
  const filtered = filterInjectorDefinitionsByTemplateUsage(injectorCatalog, usage);

  assert.deepEqual(usage, {});
  assert.deepEqual(filtered, []);
});

test("extracts injector field usage from live editor sources such as token fields and tiptap variable spans", () => {
  const usage = extractInjectorUsageFromTemplateSources([
    {
      blocks: [
        {
          type: "text",
          content:
            '<p><span data-variable-token="injector.student.name" data-category="injector">student.name</span></p>',
        },
        {
          type: "button",
          segments: [{ kind: "token", token: "injector.brand.logo_url" }],
        },
      ],
    },
    "Subject {{ injector.student.last_name }}",
    '<p><span data-variable-token="injector.student.favorite color" data-category="injector">student.favorite color</span></p>',
  ]);

  assert.deepEqual(usage, {
    student: ["name", "last_name", "favorite color"],
    brand: ["logo_url"],
  });
});

test("in visual mode ignores stale code mjml injectors and uses live builder content + metadata", () => {
  const usage = resolveTestSendInjectorUsage({
    editorMode: "visual",
    builderDocument: {
      blocks: [
        {
          type: "text",
          content:
            '<p><span data-variable-token="injector.student.name" data-category="injector">student.name</span></p>',
        },
      ],
    },
    codeMjml: "<mj-text>{{ injector.unused.field }}</mj-text>",
    metadataValues: ["From {{ injector.brand.logo_url }}"],
  });

  assert.deepEqual(usage, {
    student: ["name"],
    brand: ["logo_url"],
  });
  assert.deepEqual(Object.keys(usage), ["student", "brand"]);
});

test("in code mode uses code mjml + metadata when there is no live visual document", () => {
  const usage = resolveTestSendInjectorUsage({
    editorMode: "code",
    builderDocument: {
      blocks: [
        {
          type: "text",
          content:
            '<p><span data-variable-token="injector.student.name" data-category="injector">student.name</span></p>',
        },
      ],
    },
    codeMjml: "<mj-text>{{ injector.brand.logo_url }}</mj-text>",
    metadataValues: ["Reply {{ injector.student.last_name }}"],
  });

  assert.deepEqual(usage, {
    brand: ["logo_url"],
    student: ["last_name"],
  });
  assert.deepEqual(resolveTestSendInjectorNames({
    editorMode: "code",
    builderDocument: null,
    codeMjml: "<mj-text>{{ injector.brand.logo_url }}</mj-text>",
    metadataValues: ["Reply {{ injector.student.last_name }}"],
  }), ["brand", "student"]);
});
