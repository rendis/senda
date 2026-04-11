import test from "node:test";
import assert from "node:assert/strict";

const { readVariableTokenAttrs } = await import(
  new URL("./tiptap-variable-token.ts", import.meta.url).href
);
const {
  getVariableTokenClassName,
} = await import(new URL("./tiptap-variable-token.ts", import.meta.url).href);
const DATA_CATEGORY = "data-category";
const EVENT_USER_NAME = "event.user_name";

test("readVariableTokenAttrs preserves token, label and category from stored spans", () => {
  const element = {
    getAttribute(name: string) {
      switch (name) {
        case "data-variable-token":
          return "injector.student.name";
        case "data-label":
          return "student.name";
        case DATA_CATEGORY:
          return "injector";
        default:
          return null;
      }
    },
    textContent: "student.name",
  };

  assert.deepEqual(readVariableTokenAttrs(element), {
    token: "injector.student.name",
    label: "student.name",
    category: "injector",
  });
});

test("readVariableTokenAttrs falls back to text content when explicit label is absent", () => {
  const element = {
    getAttribute(name: string) {
      switch (name) {
        case "data-variable-token":
          return EVENT_USER_NAME;
        case DATA_CATEGORY:
          return "event";
        default:
          return null;
      }
    },
    textContent: EVENT_USER_NAME,
  };

  assert.deepEqual(readVariableTokenAttrs(element), {
    token: EVENT_USER_NAME,
    label: EVENT_USER_NAME,
    category: "event",
  });
});

test("getVariableTokenClassName constrains long tokens inside the editor width", () => {
  const injectorClasses = getVariableTokenClassName("injector");
  assert.match(injectorClasses, /\binline-flex\b/);
  assert.match(injectorClasses, /\bmax-w-full\b/);
  assert.match(injectorClasses, /\bmin-w-0\b/);
  assert.match(injectorClasses, /\boverflow-hidden\b/);
  assert.match(injectorClasses, /\btext-ellipsis\b/);
  assert.match(injectorClasses, /\bwhitespace-nowrap\b/);
  assert.match(injectorClasses, /\bborder-violet-400\b/);
});
