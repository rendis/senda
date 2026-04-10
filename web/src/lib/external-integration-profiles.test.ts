import test from "node:test";
import assert from "node:assert/strict";
import {
  DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES,
  createEmptyExternalIntegrationProfile,
  normalizeExternalIntegrationProfile,
  normalizeExternalIntegrationSettings,
  parseDelimitedEntries,
  stringifyDelimitedEntries,
} from "./external-integration-profiles.ts";

const DEMO_ORIGINS = ["https://app.example.com"];
const DEMO_HEADERS = ["x-tenant-code"];
const DEMO_AUTH_METHOD = "hmac";
const DEMO_RESOLVER = "tenant-slug";

test("parseDelimitedEntries trims, splits, and deduplicates values", () => {
  assert.deepEqual(
    parseDelimitedEntries(" https://a.example.com\nhttps://b.example.com, https://a.example.com \n"),
    ["https://a.example.com", "https://b.example.com"],
  );
});

test("stringifyDelimitedEntries keeps one entry per line", () => {
  assert.equal(
    stringifyDelimitedEntries(["https://a.example.com", "https://b.example.com"]),
    "https://a.example.com\nhttps://b.example.com",
  );
});

test("normalizeExternalIntegrationProfile fills safe defaults", () => {
  assert.deepEqual(
    normalizeExternalIntegrationProfile({}, 3),
    {
      slug: "external-profile-3",
      name: "External profile 3",
      description: "",
      enabled: false,
      auth_method_name: "",
      resolver_name: "",
      allowed_origins: [],
      allowed_headers: [],
      required_headers: [],
      capabilities: DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES,
    },
  );
});

test("createEmptyExternalIntegrationProfile is deterministic per index", () => {
  assert.deepEqual(createEmptyExternalIntegrationProfile(2).slug, "external-profile-2");
  assert.deepEqual(createEmptyExternalIntegrationProfile(2).name, "External profile 2");
});

test("normalizeExternalIntegrationSettings keeps method and resolver descriptors", () => {
  assert.deepEqual(
    normalizeExternalIntegrationSettings({
      profiles: [
        {
          slug: "embed",
          name: "Embed",
          description: "External embed profile",
          enabled: true,
          auth_method_name: DEMO_AUTH_METHOD,
          resolver_name: DEMO_RESOLVER,
          allowed_origins: DEMO_ORIGINS,
          allowed_headers: DEMO_HEADERS,
          required_headers: DEMO_HEADERS,
          capabilities: {
            list_templates: true,
            view_versions: true,
            edit_versions: false,
            publish_versions: false,
            test_send: false,
            builder_access: true,
            metadata_access: true,
            locale_access: false,
          },
        },
      ],
      available_auth_methods: [
        { name: "hmac", description: "Signed request validation" },
      ],
      available_resolvers: [
        { name: "tenant-slug", description: "Resolve by tenant and slug" },
      ],
    }),
    {
      profiles: [
        {
          slug: "embed",
          name: "Embed",
          description: "External embed profile",
          enabled: true,
          auth_method_name: DEMO_AUTH_METHOD,
          resolver_name: DEMO_RESOLVER,
          allowed_origins: DEMO_ORIGINS,
          allowed_headers: DEMO_HEADERS,
          required_headers: DEMO_HEADERS,
          capabilities: {
            list_templates: true,
            view_versions: true,
            edit_versions: false,
            publish_versions: false,
            test_send: false,
            builder_access: true,
            metadata_access: true,
            locale_access: false,
          },
        },
      ],
      available_auth_methods: [
        { name: "hmac", description: "Signed request validation" },
      ],
      available_resolvers: [
        { name: "tenant-slug", description: "Resolve by tenant and slug" },
      ],
    },
  );
});
