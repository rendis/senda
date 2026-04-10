import test from "node:test";
import assert from "node:assert/strict";

const { mjmlVarsToTiptapHtml, tiptapHtmlToMjmlVars } = await import(
  new URL("./template-variable-html.ts", import.meta.url).href
);

test("mjmlVarsToTiptapHtml converts placeholders into variable token spans", () => {
  assert.equal(
    mjmlVarsToTiptapHtml("Hola {{ injector.student.name }}"),
    'Hola <span data-variable-token="injector.student.name" data-category="injector">injector.student.name</span>',
  );
});

test("tiptapHtmlToMjmlVars converts flat variable token spans back to moustache placeholders", () => {
  const html =
    '<p><span data-variable-token="injector.student.name" data-category="injector">student.name</span></p>';

  assert.equal(tiptapHtmlToMjmlVars(html), "<p>{{injector.student.name}}</p>");
});

test("tiptapHtmlToMjmlVars converts variable token spans even when their label contains nested markup", () => {
  const html =
    '<p><span data-variable-token="injector.workspace_profile.workspace_code" data-category="injector" title="workspace_profile.workspace_code"><span class="truncate">workspace_profile.workspace_code</span></span></p>';

  assert.equal(
    tiptapHtmlToMjmlVars(html),
    "<p>{{injector.workspace_profile.workspace_code}}</p>",
  );
});
