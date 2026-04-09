import test from "node:test";
import assert from "node:assert/strict";

const {
  resolveTestSendInjectorCatalogRequest,
  resolveVisibleTestSendInjectors,
} = await import(new URL("./test-send-modal-catalog.ts", import.meta.url).href);

test("requests inherited injectors only when the template uses injector variables", () => {
  assert.deepEqual(
    resolveTestSendInjectorCatalogRequest({}),
    {
      enabled: false,
      includeInherited: false,
    },
  );

  assert.deepEqual(
    resolveTestSendInjectorCatalogRequest({
      student: ["name"],
    }),
    {
      enabled: true,
      includeInherited: true,
    },
  );
});

test("keeps inherited injectors visible when the template references them", () => {
  const injectors = resolveVisibleTestSendInjectors(
    [
      {
        name: "student",
        description: "Inherited from global",
        fields: [
          {
            field_name: "name",
            position: 0,
          },
          {
            field_name: "ignored",
            position: 1,
          },
        ],
      },
    ],
    {
      student: ["name"],
    },
  );

  assert.deepEqual(injectors, [
    {
      name: "student",
      description: "Inherited from global",
      fields: [
        {
          field_name: "name",
          position: 0,
        },
      ],
    },
  ]);
});
