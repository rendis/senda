import test from "node:test";
import assert from "node:assert/strict";

const {
  buildInjectorTooltipSections,
  injectorFieldTypeIconName,
  injectorFieldTypeLabel,
} = await import(new URL("./injector-variable-presentation.ts", import.meta.url).href);

test("injectorFieldTypeLabel returns user-facing labels", () => {
  assert.equal(injectorFieldTypeLabel("text"), "Text");
  assert.equal(injectorFieldTypeLabel("number"), "Number");
  assert.equal(injectorFieldTypeLabel("bool"), "Boolean");
  assert.equal(injectorFieldTypeLabel("img"), "Image");
  assert.equal(injectorFieldTypeLabel("url"), "URL");
  assert.equal(injectorFieldTypeLabel("html"), "HTML");
});

test("injectorFieldTypeIconName maps field types to stable icon names", () => {
  assert.equal(injectorFieldTypeIconName("text"), "type");
  assert.equal(injectorFieldTypeIconName("number"), "hash");
  assert.equal(injectorFieldTypeIconName("bool"), "toggle-right");
  assert.equal(injectorFieldTypeIconName("img"), "image");
  assert.equal(injectorFieldTypeIconName("url"), "link");
  assert.equal(injectorFieldTypeIconName("html"), "code-2");
});

test("buildInjectorTooltipSections exposes descriptive metadata for builder tooltips", () => {
  assert.deepEqual(
    buildInjectorTooltipSections({
      fullLabel: "brand.company_name",
      fieldLabel: "company_name",
      fieldType: "text",
      fieldDescription: "Brand/company display name",
      injectorDescription:
        "Static brand assets and legal/footer content for the active workspace.",
      static: true,
      inheritedFromSystem: true,
      source: "code",
    }),
    {
      name: "brand.company_name",
      description: "Brand/company display name",
      injectorDescription:
        "Static brand assets and legal/footer content for the active workspace.",
      details: [
        { label: "Static", value: "Yes" },
        { label: "Type", value: "Text" },
        { label: "Inherited", value: "Yes" },
        { label: "Source", value: "Code" },
      ],
    },
  );
});
