import test from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("API keys help article uses a reusable endpoint reference component", () => {
  assert.equal(
    existsSync(join(root, "web/src/components/help/api-endpoint-reference.tsx")),
    true,
    "API endpoint reference component should exist",
  );

  const mdxComponents = read("web/src/components/help/mdx-components.tsx");
  const apiKeysGuide = read("web/content/help/api-keys.mdx");

  assert.match(
    mdxComponents,
    /ApiEndpointReference/,
    "MDX components registry should expose the API endpoint reference component",
  );

  assert.match(
    apiKeysGuide,
    /<ApiEndpointReference\s*\/>/,
    "API keys help guide should render the reusable endpoint reference block",
  );
});

test("API endpoint reference renders collapsible sections with complete endpoint details", () => {
  const component = read("web/src/components/help/api-endpoint-reference.tsx");

  assert.match(
    component,
    /data-plane-openapi\.json/,
    "Reference component should read from the filtered data-plane OpenAPI spec",
  );

  assert.match(
    component,
    /<details/,
    "Each endpoint should be rendered in a collapsible details block",
  );

  assert.match(
    component,
    /<summary/,
    "Each collapsible endpoint block should have a summary row",
  );

  for (const label of [
    "Authentication",
    "Headers",
    "Path parameters",
    "Query parameters",
    "Request body",
    "Responses",
    "None",
  ]) {
    assert.match(
      component,
      new RegExp(label),
      `Reference component should render the ${label} section or fallback`,
    );
  }
});
