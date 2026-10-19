import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const schemaDiff = vi.fn(async () => ({
  source_environment_id: "00000000-0000-0000-0000-000000000001",
  target_environment_id: "00000000-0000-0000-0000-000000000001",
  source_run_id: "r1",
  target_run_id: "r2",
  partial: false,
  summary: {
    match: 0,
    drift: 0,
    only_source: 0,
    only_target: 0,
    unknown: 0,
    total: 0,
  },
  objects: [],
  total: 0,
  limit: 50,
  offset: 0,
}));
const compare = vi.fn();
const auditRuns = vi.fn(async () => ({ items: [] }));

vi.mock("../services/api", () => ({
  api: {
    schemaDiff,
    compare,
    auditRuns,
    hasRole: () => false,
  },
}));

vi.mock("../context/AppContext", () => ({
  useApp: () => ({
    environmentId: "00000000-0000-0000-0000-000000000001",
    environments: [
      { id: "00000000-0000-0000-0000-000000000001", name: "Demo" },
    ],
    setEnvironmentId: () => undefined,
    setSection: () => undefined,
  }),
}));

describe("SchemaDriftPageUsesRealRuns", () => {
  beforeEach(() => {
    schemaDiff.mockClear();
    compare.mockClear();
  });

  it("does not render demo payloads or call POST /compare with fixed body", async () => {
    const { SchemaDriftPage } = await import("./SchemaDriftPage");
    const html = renderToStaticMarkup(<SchemaDriftPage />);
    expect(html).not.toContain("demo payloads");
    expect(html).not.toContain("Comparar (demo payloads)");
    expect(html).not.toContain("fp-orders-v1");
    expect(html).toContain("Comparar");
    expect(compare).not.toHaveBeenCalled();
  });
});
