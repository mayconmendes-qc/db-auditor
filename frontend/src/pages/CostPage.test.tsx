import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../context/AppContext", () => ({
  useApp: () => ({ environmentId: "00000000-0000-0000-0000-000000000001" }),
}));

vi.mock("../services/api", () => ({
  api: {
    costRanking: vi.fn(async () => ({
      items: [],
      priced: false,
      reason: "não estimado — preço não configurado",
    })),
    costSeries: vi.fn(async () => ({ items: [] })),
    costRankingExport: vi.fn(),
    hasRole: () => false,
  },
}));

describe("CostPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders cost heading", async () => {
    const { CostPage } = await import("./CostPage");
    const html = renderToStaticMarkup(<CostPage />);
    expect(html).toContain("Custo estimado");
  });
});
