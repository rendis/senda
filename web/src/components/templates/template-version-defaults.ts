export const DEFAULT_MJML = `<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hello 👋</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`;

export function buildDefaultEditorData(createId: () => string) {
  return {
    version: 1,
    blocks: [
      {
        id: createId(),
        type: "text",
        content: "<p>Hello 👋</p>",
        align: "left",
      },
    ],
  };
}
