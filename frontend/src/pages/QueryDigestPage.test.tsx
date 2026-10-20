import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hasRole = vi.fn();

vi.mock("../services/api", () => ({
  api: {
    hasRole: (role: string) => hasRole(role),
    queryDigests: vi.fn(async () => ({
      items: [],
      page: { limit: 50, offset: 0, total: 0, has_more: false },
    })),
    queryDigestConfig: vi.fn(async () => ({
      environment_id: "env",
      window_days: 7,
      thresholds: {},
      delivery_enabled: true,
    })),
    notificationDestinations: vi.fn(async () => ({
      items: [],
      using_global_fallback: true,
    })),
  },
}));

vi.mock("../context/AppContext", () => ({
  useApp: () => ({
    environmentId: "00000000-0000-0000-0000-000000000001",
    environments: [
      { id: "00000000-0000-0000-0000-000000000001", name: "Demo" },
    ],
    setEnvironmentId: () => undefined,
    openFinding: () => undefined,
  }),
}));

vi.mock("../hooks/useLatestSignal", () => ({
  useLatestSignal: () => () => ({ aborted: false }),
}));

describe("QueryDigestPage operator gating", () => {
  beforeEach(() => {
    hasRole.mockReset();
  });

  it("hides operator controls for viewer", async () => {
    hasRole.mockImplementation(() => false);
    const { QueryDigestPage } = await import("./QueryDigestPage");
    const html = renderToStaticMarkup(<QueryDigestPage />);
    expect(html).toContain("só para operadores");
    expect(html).not.toContain("Gerar agora");
    expect(html).not.toContain("Substituir destinatários");
  });

  it("shows operator controls for operator", async () => {
    hasRole.mockImplementation((role: string) => role === "operator");
    // Reset module so the page re-reads hasRole on import... page calls hasRole at render.
    const { QueryDigestPage } = await import("./QueryDigestPage");
    const html = renderToStaticMarkup(<QueryDigestPage />);
    expect(html).toContain("Gerar agora");
    expect(html).toContain("Destinatários do ambiente");
  });
});
