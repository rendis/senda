import test from "node:test";
import assert from "node:assert/strict";

const { renderTextBlockToMjml } = await import(
  new URL("./text-block-mjml.ts", import.meta.url).href
);

test("renderTextBlockToMjml uses block align when content has no paragraph-level alignment", () => {
  const html = "<p>Hello</p>";

  assert.equal(renderTextBlockToMjml(html, "center"), '<mj-text align="center"><p>Hello</p></mj-text>');
});

test("renderTextBlockToMjml preserves mixed paragraph alignment by omitting outer mj-text align", () => {
  const html = [
    '<p style="text-align: center;">Hello <span data-variable-token="injector.workspace_profile.workspace_code" data-category="injector">injector.workspace_profile.workspace_code</span></p>',
    '<p style="text-align: center;"><span data-variable-token="injector.workspace_profile.environment_badge_html" data-category="injector">injector.workspace_profile.environment_badge_html</span></p>',
    '<p><span data-variable-token="injector.student.name" data-category="injector">injector.student.name</span></p>',
    '<p><span data-variable-token="injector.student.status" data-category="injector">injector.student.status</span></p>',
  ].join("");

  assert.equal(
    renderTextBlockToMjml(html, "center"),
    [
      "<mj-text>",
      "<p style=\"text-align: center;\">Hello {{injector.workspace_profile.workspace_code}}</p>",
      "<p style=\"text-align: center;\">{{injector.workspace_profile.environment_badge_html}}</p>",
      "<p>{{injector.student.name}}</p>",
      "<p>{{injector.student.status}}</p>",
      "</mj-text>",
    ].join(""),
  );
});
