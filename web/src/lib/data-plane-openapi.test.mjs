import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const specPath = path.join(__dirname, "data-plane-openapi.json");

function loadSpec() {
  return JSON.parse(fs.readFileSync(specPath, "utf8"));
}

test("data plane API spec includes every API key endpoint", () => {
  const spec = loadSpec();

  assert.deepEqual(Object.keys(spec.paths).sort(), [
    "/api/v1/emails",
    "/api/v1/emails/export",
    "/api/v1/emails/{tracking_id}",
    "/api/v1/emails/{tracking_id}/events",
    "/api/v1/send",
  ]);
});

test("every operation requires the workspace API key security scheme", () => {
  const spec = loadSpec();

  for (const pathItem of Object.values(spec.paths)) {
    for (const operation of Object.values(pathItem)) {
      assert.deepEqual(operation.security, [{ WorkspaceAPIKeyBearer: [] }]);
    }
  }
});

test("list emails exposes all supported query filters", () => {
  const spec = loadSpec();
  const operation = spec.paths["/api/v1/emails"].get;
  const parameterNames = operation.parameters.map((parameter) => parameter.name).sort();

  assert.deepEqual(parameterNames, [
    "adapter_id",
    "cursor",
    "external_id",
    "limit",
    "recipient",
    "since",
    "status",
    "template_type",
    "until",
  ]);
});

test("export emails documents CSV output and the same filters as list", () => {
  const spec = loadSpec();
  const operation = spec.paths["/api/v1/emails/export"].get;
  const parameterNames = operation.parameters.map((parameter) => parameter.name).sort();

  assert.equal(
    operation.responses["200"].content["text/csv"].schema.type,
    "string",
  );
  assert.deepEqual(parameterNames, [
    "adapter_id",
    "cursor",
    "external_id",
    "limit",
    "recipient",
    "since",
    "status",
    "template_type",
    "until",
  ]);
});

test("send endpoint documents JSON request body", () => {
  const spec = loadSpec();
  const operation = spec.paths["/api/v1/send"].post;

  assert.equal(operation.requestBody.required, true);
  assert.ok(operation.requestBody.content["application/json"]);
});
