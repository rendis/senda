import test from "node:test";
import assert from "node:assert/strict";

const {
  buildSmtpConfig,
  hasExistingSmtpAuth,
  validateSmtpAuthFields,
} = await import(new URL("./smtp-config-form-model.ts", import.meta.url).href);

const SMTP_HOST = "smtp.example.com";
const SMTP_PORT = "587";
const SMTP_TLS_MODE = "starttls";
const SMTP_AUTH_MODE = "plain";
const EXISTING_AUTH_CONFIG = {
  host: SMTP_HOST,
  port: SMTP_PORT,
  tls_mode: SMTP_TLS_MODE,
  auth_mode: SMTP_AUTH_MODE,
};

test("smtp auth validation requires username and password together on create", () => {
  assert.equal(
    validateSmtpAuthFields({
      username: "apikey",
      password: "",
      isEdit: false,
      hasExistingAuth: false,
      clearAuth: false,
    }).valid,
    false,
  );

  assert.equal(
    validateSmtpAuthFields({
      username: "",
      password: "secret",
      isEdit: false,
      hasExistingAuth: false,
      clearAuth: false,
    }).valid,
    false,
  );
});

test("smtp auth validation preserves existing password when username is supplied in edit mode", () => {
  assert.equal(
    validateSmtpAuthFields({
      username: "apikey",
      password: "",
      isEdit: true,
      hasExistingAuth: true,
      clearAuth: false,
    }).valid,
    true,
  );
});

test("smtp config builder omits auth fields when editing non-auth settings", () => {
  assert.deepEqual(
    buildSmtpConfig({
      host: "smtp-new.example.com",
      port: "587",
      tlsMode: "starttls",
      authMode: "plain",
      username: "",
      password: "",
      isEdit: true,
      clearAuth: false,
      previousConfig: {
        ...EXISTING_AUTH_CONFIG,
      },
    }),
    {
      host: "smtp-new.example.com",
      port: Number(SMTP_PORT),
      tls_mode: SMTP_TLS_MODE,
    },
  );
});

test("smtp config builder sends explicit empty username to clear existing auth", () => {
  assert.deepEqual(
    buildSmtpConfig({
      host: SMTP_HOST,
      port: SMTP_PORT,
      tlsMode: SMTP_TLS_MODE,
      authMode: SMTP_AUTH_MODE,
      username: "",
      password: "",
      isEdit: true,
      clearAuth: true,
      previousConfig: EXISTING_AUTH_CONFIG,
    }),
    {
      host: SMTP_HOST,
      port: Number(SMTP_PORT),
      tls_mode: SMTP_TLS_MODE,
      username: "",
    },
  );
});

test("smtp config builder preserves password when edit username is non-empty and password is blank", () => {
  assert.deepEqual(
    buildSmtpConfig({
      host: SMTP_HOST,
      port: SMTP_PORT,
      tlsMode: SMTP_TLS_MODE,
      authMode: "login",
      username: "apikey",
      password: "",
      isEdit: true,
      clearAuth: false,
      previousConfig: EXISTING_AUTH_CONFIG,
    }),
    {
      host: SMTP_HOST,
      port: Number(SMTP_PORT),
      tls_mode: SMTP_TLS_MODE,
      auth_mode: "login",
      username: "apikey",
    },
  );
});

test("smtp config builder returns undefined when edit smtp config is unchanged", () => {
  assert.equal(
    buildSmtpConfig({
      host: SMTP_HOST,
      port: SMTP_PORT,
      tlsMode: SMTP_TLS_MODE,
      authMode: SMTP_AUTH_MODE,
      username: "",
      password: "",
      isEdit: true,
      clearAuth: false,
      previousConfig: EXISTING_AUTH_CONFIG,
    }),
    undefined,
  );
});

test("smtp auth metadata is detected from public config metadata", () => {
  assert.equal(hasExistingSmtpAuth({ auth_mode: "plain" }), true);
  assert.equal(hasExistingSmtpAuth({}), false);
  assert.equal(hasExistingSmtpAuth(undefined), false);
});
