import test from "node:test";
import assert from "node:assert/strict";

const {
  normalizeVariableToken,
  parseContentToSegments,
  renderSegmentsToText,
  getTokenChipText,
} = await import(new URL("./template-token-segments.ts", import.meta.url).href);

test("normalizeVariableToken removes moustache syntax and collapses whitespace", () => {
  assert.equal(
    normalizeVariableToken("  {{   injector.student.name   }}  "),
    "injector.student.name",
  );
});

test("parseContentToSegments turns placeholders into token segments with readable labels", () => {
  const segments = parseContentToSegments(
    "Hola {{ injector.student.name }} desde {{ event.workspace_name }}",
  );

  assert.equal(segments.length, 4);
  assert.deepEqual(
    segments
      .filter((segment: { kind: string }) => segment.kind === "token")
      .map((segment: { token: string; label: string; category: string }) => ({
        token: segment.token,
        label: segment.label,
        category: segment.category,
      })),
    [
      {
        token: "injector.student.name",
        label: "injector.student.name",
        category: "injector",
      },
      {
        token: "event.workspace_name",
        label: "event.workspace_name",
        category: "event",
      },
    ],
  );
});

test("renderSegmentsToText preserves placeholder syntax for storage", () => {
  const segments = parseContentToSegments(
    "Equipo {{ injector.school.name }} te saluda",
  );

  assert.equal(
    renderSegmentsToText(segments),
    "Equipo {{ injector.school.name }} te saluda",
  );
});

test("getTokenChipText prefers the human-readable label instead of raw moustache", () => {
  assert.equal(
    getTokenChipText({
      token: "injector.school.name",
      label: "school.name",
    }),
    "school.name",
  );
});
