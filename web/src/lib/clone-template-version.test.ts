import test from "node:test";
import assert from "node:assert/strict";

const { cloneTemplateVersion } = await import(
  new URL("./clone-template-version.ts", import.meta.url).href,
);

test("cloneTemplateVersion posts to the explicit clone endpoint and returns the created draft", async () => {
  let receivedPath = "";
  const expected = {
    id: "version-cloned",
    template_id: "template-1",
    version_number: 3,
    status: "draft",
    subject: "Cloned subject",
    from_name: "Senda",
    body_mjml: "<mjml></mjml>",
    default_locale: "en",
    created_at: "2026-04-09T00:00:00Z",
  };

  const api = {
    post(path: string) {
      receivedPath = path;
      return {
        json: async () => expected,
      };
    },
  };

  const result = await cloneTemplateVersion(
    api as never,
    "/api/v1/manage/tenants/acme/workspaces/default",
    "template-1",
    "version-2"
  );

  assert.equal(
    receivedPath,
    "/api/v1/manage/tenants/acme/workspaces/default/templates/template-1/versions/version-2/clone"
  );
  assert.deepEqual(result, expected);
});
