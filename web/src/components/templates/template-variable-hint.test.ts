import test from "node:test";
import assert from "node:assert/strict";

const { buildInjectorVariableHint } = await import(
  new URL("./template-variable-hint.ts", import.meta.url).href
);

const FALLBACK_HINT = "Injector variable";
const INJECTOR_DESCRIPTION = "Datos del estudiante";

test("prefers the field description when it exists", () => {
  assert.equal(
    buildInjectorVariableHint({
      fieldDescription: "Nombre del alumno",
      injectorDescription: INJECTOR_DESCRIPTION,
      fallbackHint: FALLBACK_HINT,
    }),
    `Nombre del alumno · ${INJECTOR_DESCRIPTION}`,
  );
});

test("falls back to injector description when the field description is missing", () => {
  assert.equal(
    buildInjectorVariableHint({
      injectorDescription: INJECTOR_DESCRIPTION,
      fallbackHint: FALLBACK_HINT,
    }),
    INJECTOR_DESCRIPTION,
  );
});

test("avoids duplicating identical descriptions", () => {
  assert.equal(
    buildInjectorVariableHint({
      fieldDescription: INJECTOR_DESCRIPTION,
      injectorDescription: INJECTOR_DESCRIPTION,
      fallbackHint: FALLBACK_HINT,
    }),
    INJECTOR_DESCRIPTION,
  );
});

test("uses the fallback hint only when no description exists", () => {
  assert.equal(
    buildInjectorVariableHint({
      fieldDescription: "   ",
      injectorDescription: "",
      fallbackHint: FALLBACK_HINT,
    }),
    FALLBACK_HINT,
  );
});
