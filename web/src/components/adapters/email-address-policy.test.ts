import test from "node:test";
import assert from "node:assert/strict";

const { normalizeReasonableEmailAddress } = await import(
  new URL("./email-address-policy.ts", import.meta.url).href
);

test("normalizes valid sender email addresses", () => {
  assert.equal(
    normalizeReasonableEmailAddress(" sender@example.com "),
    "sender@example.com",
  );
});

test("rejects malformed sender email addresses", () => {
  assert.equal(normalizeReasonableEmailAddress("sender"), null);
  assert.equal(normalizeReasonableEmailAddress("sender@example"), null);
  assert.equal(normalizeReasonableEmailAddress("sender@@example.com"), null);
  assert.equal(normalizeReasonableEmailAddress("sender example@example.com"), null);
});
