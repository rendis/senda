import test from "node:test";
import assert from "node:assert/strict";

const { shouldRestoreDialogFocus } = await import(
  new URL("./form-dialog-focus.ts", import.meta.url).href,
);

test("restores focus when previous element is connected and enabled", () => {
  assert.equal(
    shouldRestoreDialogFocus({
      isConnected: true,
      disabled: false,
      ariaHidden: null,
    }),
    true,
  );
});

test("does not restore focus to disabled elements", () => {
  assert.equal(
    shouldRestoreDialogFocus({
      isConnected: true,
      disabled: true,
      ariaHidden: null,
    }),
    false,
  );
});

test("does not restore focus to aria-hidden elements", () => {
  assert.equal(
    shouldRestoreDialogFocus({
      isConnected: true,
      disabled: false,
      ariaHidden: "true",
    }),
    false,
  );
});

test("does not restore focus to disconnected elements", () => {
  assert.equal(
    shouldRestoreDialogFocus({
      isConnected: false,
      disabled: false,
      ariaHidden: null,
    }),
    false,
  );
});
