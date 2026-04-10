import test from "node:test";
import assert from "node:assert/strict";

const {
  buildInjectorRequest,
  canEditInjectorSchema,
  emptyInjectorFormField,
  injectorDefinitionToFormValues,
  parseInjectorFieldValue,
  resolveUpdatedInjectorSelection,
  serializeInjectorFieldValue,
  supportsInjectorManagementScope,
} = await import(new URL("./injector-form-model.ts", import.meta.url).href);

type InjectorDefinition = import("@/types/injectors").InjectorDefinition;
type InjectorFormFieldEntry = import("./injector-form-model").InjectorFormFieldEntry;

const FULL_NAME = "full name";
const ADA_LOVELACE = "Ada Lovelace";
const STUDENT_PROFILE = "student profile";

test("injectorDefinitionToFormValues hydrates edit mode state preserving order and defaults", () => {
  const injector: Pick<InjectorDefinition, "name" | "description" | "fields"> = {
    name: "student",
    description: "Student profile",
    fields: [
      {
        field_name: "age",
        field_type: "number",
        position: 1,
        default_value: 18,
        allow_overwrite: false,
      },
      {
        field_name: FULL_NAME,
        field_type: "text",
        position: 0,
        description: "Shown in greeting",
        default_value: ADA_LOVELACE,
        allow_overwrite: true,
      },
    ],
  };

  const values = injectorDefinitionToFormValues(injector);

  assert.equal(values.name, "student");
  assert.equal(values.description, "Student profile");
  assert.equal(values.fields.length, 2);
  assert.equal(values.fields[0].form_id.length > 0, true);
  assert.equal(values.fields[1].form_id.length > 0, true);
  assert.notEqual(values.fields[0].form_id, values.fields[1].form_id);
  assert.deepEqual(
    values.fields.map(
      ({
        field_name,
        field_type,
        description,
        default_value,
        allow_overwrite,
      }: InjectorFormFieldEntry) => ({
        field_name,
        field_type,
        description,
        default_value,
        allow_overwrite,
      }),
    ),
    [
      {
        field_name: "full name",
        field_type: "text",
        description: "Shown in greeting",
        default_value: ADA_LOVELACE,
        allow_overwrite: true,
      },
      {
        field_name: "age",
        field_type: "number",
        description: "",
        default_value: "18",
        allow_overwrite: false,
      },
    ],
  );
});

test("buildInjectorRequest serializes edited fields with index-based positions", () => {
  const request = buildInjectorRequest({
    name: STUDENT_PROFILE,
    description: "  ",
    fields: [
      {
        form_id: "field-1",
        field_name: FULL_NAME,
        field_type: "text",
        description: "Primary label",
        default_value: ADA_LOVELACE,
        allow_overwrite: true,
      },
      {
        form_id: "field-2",
        field_name: "is active",
        field_type: "bool",
        description: "",
        default_value: "false",
        allow_overwrite: false,
      },
    ],
  });

  assert.deepEqual(request, {
    name: STUDENT_PROFILE,
    description: undefined,
    fields: [
      {
        field_name: "full name",
        field_type: "text",
        description: "Primary label",
        position: 0,
        default_value: ADA_LOVELACE,
        allow_overwrite: true,
      },
      {
        field_name: "is active",
        field_type: "bool",
        description: undefined,
        position: 1,
        default_value: false,
        allow_overwrite: false,
      },
    ],
  });
});

test("scope helpers allow management only in global and workspace owned views", () => {
  assert.equal(supportsInjectorManagementScope("global"), true);
  assert.equal(supportsInjectorManagementScope("workspace"), true);
  assert.equal(supportsInjectorManagementScope("tenant"), false);

  assert.equal(canEditInjectorSchema("global", { workspace_id: undefined }), true);
  assert.equal(canEditInjectorSchema("global", { workspace_id: "ws-1" }), false);
  assert.equal(canEditInjectorSchema("workspace", { workspace_id: "ws-1" }), true);
  assert.equal(canEditInjectorSchema("workspace", { workspace_id: undefined }), false);
  assert.equal(canEditInjectorSchema("tenant", { workspace_id: undefined }), false);
});

test("serialization helpers preserve bool defaults and spaced field names use regular request parsing", () => {
  assert.equal(serializeInjectorFieldValue("bool", undefined), "false");
  assert.equal(parseInjectorFieldValue("number", "42"), 42);
  assert.equal(parseInjectorFieldValue("text", ""), undefined);
  assert.equal(emptyInjectorFormField().field_name, "");
  assert.equal(emptyInjectorFormField().form_id.length > 0, true);
  assert.equal(
    resolveUpdatedInjectorSelection({ name: STUDENT_PROFILE }),
    STUDENT_PROFILE,
  );
});
