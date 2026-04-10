import test from "node:test";
import assert from "node:assert/strict";

import {
  buildExternalEmbedApiRequest,
  captureExternalEmbedTokenFromSearch,
  clearExternalEmbedToken,
  EXTERNAL_EMBED_TOKEN_HEADER,
  EXTERNAL_ENVIRONMENT_HEADER,
  EXTERNAL_EMBED_TOKEN_STORAGE_KEY,
  EXTERNAL_TENANT_HEADER,
  isExternalEmbedPath,
  isExternalEmbedRuntimeReady,
  readExternalEmbedToken,
  resolveScopedPathFromParams,
  resolveWorkspacePoliciesPathFromParams,
  storeExternalEmbedToken,
  stripExternalEmbedTokenFromSearch,
} from "./external-api-context.ts";

const SIGNED_TOKEN = "signed-token";

function createStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    getItem(key: string) {
      return data.get(key) ?? null;
    },
    setItem(key: string, value: string) {
      data.set(key, value);
    },
    removeItem(key: string) {
      data.delete(key);
    },
  };
}

test("resolveScopedPathFromParams returns external path for embed params", () => {
  assert.equal(
    resolveScopedPathFromParams({
      profileSlug: "partner-portal",
      tenantCode: "acme",
      workspaceCode: "marketing",
    }),
    "external/partner-portal/tenants/acme/workspaces/marketing",
  );
});

test("resolveScopedPathFromParams keeps management paths for regular scopes", () => {
  assert.equal(
    resolveScopedPathFromParams({
      tenantCode: "acme",
      workspaceCode: "marketing",
    }),
    "manage/environments/prod/tenants/acme/workspaces/marketing",
  );
  assert.equal(
    resolveScopedPathFromParams({
      tenantCode: "acme",
      workspaceCode: "marketing",
      environment: "test",
    }),
    "manage/environments/test/tenants/acme/workspaces/marketing",
  );
  assert.equal(
    resolveScopedPathFromParams({ tenantCode: "acme" }),
    "manage/tenants/acme",
  );
  assert.equal(resolveScopedPathFromParams({}), "manage/global");
});

test("resolveWorkspacePoliciesPathFromParams returns external policies path for embed params", () => {
  assert.equal(
    resolveWorkspacePoliciesPathFromParams({
      profileSlug: "partner-portal",
      tenantCode: "acme",
      workspaceCode: "marketing",
    }),
    "external/partner-portal/tenants/acme/workspaces/marketing/policies",
  );
});

test("resolveWorkspacePoliciesPathFromParams returns management path outside embed", () => {
  assert.equal(
    resolveWorkspacePoliciesPathFromParams({
      tenantCode: "acme",
      workspaceCode: "marketing",
    }),
    "manage/environments/prod/tenants/acme/workspaces/marketing/policies",
  );
  assert.equal(
    resolveWorkspacePoliciesPathFromParams({
      tenantCode: "acme",
      workspaceCode: "marketing",
      environment: "test",
    }),
    "manage/environments/test/tenants/acme/workspaces/marketing/policies",
  );
  assert.equal(resolveWorkspacePoliciesPathFromParams({ tenantCode: "acme" }), null);
});

test("stripExternalEmbedTokenFromSearch removes token but preserves other params", () => {
  assert.equal(
    stripExternalEmbedTokenFromSearch(`?templateId=1&versionId=2&token=${SIGNED_TOKEN}`),
    "?templateId=1&versionId=2",
  );

  assert.equal(stripExternalEmbedTokenFromSearch(`token=${SIGNED_TOKEN}`), "");
});

test("captureExternalEmbedTokenFromSearch stores token and returns cleaned search", () => {
  const storage = createStorage();
  const result = captureExternalEmbedTokenFromSearch(
    `?templateId=1&token=${SIGNED_TOKEN}`,
    storage,
  );

  assert.equal(result.token, SIGNED_TOKEN);
  assert.equal(result.cleanedSearch, "?templateId=1");
  assert.equal(readExternalEmbedToken(storage), SIGNED_TOKEN);
});

test("isExternalEmbedPath only matches embed routes", () => {
  assert.equal(isExternalEmbedPath("/embed/partner-portal/t/acme"), true);
  assert.equal(isExternalEmbedPath("/t/acme"), false);
});

test("isExternalEmbedRuntimeReady only returns true when session token exists", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  assert.equal(isExternalEmbedRuntimeReady("/embed/partner-portal/t/acme", storage), true);
  assert.equal(isExternalEmbedRuntimeReady("/t/acme", storage), false);
  assert.equal(isExternalEmbedRuntimeReady("/embed/partner-portal/t/acme"), false);
});

test("buildExternalEmbedApiRequest adds the dedicated external headers for external api requests", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  const request = new Request(
    "https://senda.tether.education/api/v1/external/partner-portal/bootstrap",
    {
      headers: {
        accept: "application/json",
      },
    },
  );

  const nextRequest = buildExternalEmbedApiRequest(
    request,
    "/embed/partner-portal/t/acme/w/marketing/templates/newsletter/edit",
    storage,
  );

  assert.ok(nextRequest);
  assert.equal(nextRequest?.headers.get(EXTERNAL_EMBED_TOKEN_HEADER), SIGNED_TOKEN);
  assert.equal(nextRequest?.headers.get(EXTERNAL_TENANT_HEADER), "acme");
  assert.equal(nextRequest?.headers.get(EXTERNAL_ENVIRONMENT_HEADER), "prod");
  assert.equal(nextRequest?.headers.get("accept"), "application/json");
});

test("buildExternalEmbedApiRequest derives environment header from embed path", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  const request = new Request(
    "https://senda.tether.education/api/v1/external/partner-portal/bootstrap",
    {
      headers: {
        accept: "application/json",
      },
    },
  );

  const nextRequest = buildExternalEmbedApiRequest(
    request,
    "/embed/partner-portal/environments/test/t/acme/w/marketing/templates/newsletter/edit",
    storage,
  );

  assert.ok(nextRequest);
  assert.equal(nextRequest?.headers.get(EXTERNAL_ENVIRONMENT_HEADER), "test");
  assert.equal(nextRequest?.headers.get(EXTERNAL_TENANT_HEADER), "acme");
});



test("buildExternalEmbedApiRequest mutates POST requests in place when the URL does not change", async () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  const request = new Request(
    "https://senda.tether.education/api/v1/external/partner-portal/tenants/acme/workspaces/marketing/templates/123/preview-mjml",
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify({ mjml: "<mjml />" }),
    },
  );

  const nextRequest = buildExternalEmbedApiRequest(
    request,
    "/embed/partner-portal/t/acme/w/marketing/templates/newsletter/edit",
    storage,
    `?templateId=123&versionId=456&token=${SIGNED_TOKEN}`,
  );

  assert.ok(nextRequest);
  assert.equal(nextRequest, request);
  assert.equal(nextRequest?.headers.get(EXTERNAL_EMBED_TOKEN_HEADER), SIGNED_TOKEN);
  assert.equal(nextRequest?.headers.get(EXTERNAL_TENANT_HEADER), "acme");
  assert.equal(nextRequest?.headers.get(EXTERNAL_ENVIRONMENT_HEADER), "prod");
  assert.equal(await nextRequest?.clone().text(), JSON.stringify({ mjml: "<mjml />" }));
});

test("buildExternalEmbedApiRequest forwards embed fallback query params to external api requests", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  const request = new Request(
    "https://senda.tether.education/api/v1/external/partner-portal/tenants/acme/workspaces/missing/templates/123/versions/456",
  );

  const nextRequest = buildExternalEmbedApiRequest(
    request,
    "/embed/partner-portal/t/acme/w/missing/templates/newsletter/edit",
    storage,
    "?templateId=123&versionId=456&fallback=system&token=signed-token",
  );

  assert.ok(nextRequest);
  const url = new URL(nextRequest!.url);
  assert.equal(url.searchParams.get("fallback"), "system");
  assert.equal(url.searchParams.get("token"), null);
  assert.equal(url.searchParams.get("templateId"), null);
});

test("buildExternalEmbedApiRequest ignores non-embed pages and non-external requests", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  const request = new Request("https://senda.tether.education/api/v1/manage/tenants");

  assert.equal(
    buildExternalEmbedApiRequest(
      request,
      "/t/acme",
      storage,
    ),
    undefined,
  );

  assert.equal(
    buildExternalEmbedApiRequest(
      new Request("https://senda.tether.education/api/v1/manage/tenants"),
      "/embed/partner-portal/t/acme/w/marketing/templates/newsletter/edit",
      storage,
    ),
    undefined,
  );
});

test("clearExternalEmbedToken removes the stored token", () => {
  const storage = createStorage({ [EXTERNAL_EMBED_TOKEN_STORAGE_KEY]: SIGNED_TOKEN });
  assert.equal(clearExternalEmbedToken(storage), true);
  assert.equal(readExternalEmbedToken(storage), null);
  assert.equal(storeExternalEmbedToken("another-token", storage), true);
  assert.equal(readExternalEmbedToken(storage), "another-token");
});
