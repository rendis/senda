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

test("API endpoint reference renders compact responses with expandable deep detail", () => {
  const component = read("web/src/components/help/api-endpoint-reference.tsx");

  for (const label of [
    "Schema summary",
    "Example",
    "View full schema fields",
    "Hide full schema fields",
  ]) {
    assert.match(
      component,
      new RegExp(label),
      `Response rendering should include the ${label} affordance`,
    );
  }

  assert.match(
    component,
    /getSchemaSummaryRows/,
    "Response rendering should summarize schema fields instead of only dumping the full flattened matrix",
  );

  assert.match(
    component,
    /getExampleValue/,
    "Response rendering should generate example payloads when the OpenAPI spec does not provide one",
  );
});

test("API endpoint reference makes each response code card collapsible", () => {
  const component = read("web/src/components/help/api-endpoint-reference.tsx");

  assert.match(
    component,
    /<details className=\"group\/status-response/,
    "Each response code card should be wrapped in its own collapsible details element",
  );

  assert.match(
    component,
    /group-open\/status-response:rotate-90/,
    "Each response code summary should show collapsible affordance state",
  );

  for (const label of [
    "View response details",
    "Hide response details",
  ]) {
    assert.match(
      component,
      new RegExp(label),
      `Each response card should expose the ${label} label`,
    );
  }
});
