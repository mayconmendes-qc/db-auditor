import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const session = new Map<string, string>();

beforeEach(() => {
  session.clear();
  vi.resetModules();
  vi.stubGlobal("window", {
    sessionStorage: {
      getItem: (key: string) => session.get(key) ?? null,
      setItem: (key: string, value: string) => session.set(key, value),
      removeItem: (key: string) => session.delete(key),
    },
    dispatchEvent: vi.fn(),
  });
});

afterEach(() => vi.unstubAllGlobals());

describe("session restoration", () => {
  it("keeps a login in the same tab and verifies it after reload", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(
      Response.json({
        token: "valid-token",
        user: { username: "admin", role: "operator" },
      }),
    );
    vi.stubGlobal("fetch", fetch);
    const { api } = await import("./api");
    await api.login("admin", "password");
    expect(session.get("db-auditor:session")).toBe("valid-token");

    vi.resetModules();
    fetch.mockResolvedValueOnce(
      Response.json({ user: { username: "admin", role: "operator" } }),
    );
    const { api: reloaded } = await import("./api");
    await expect(reloaded.restoreSession()).resolves.toEqual({
      username: "admin",
      role: "operator",
    });
    expect(fetch.mock.calls[1][1].headers.Authorization).toBe(
      "Bearer valid-token",
    );
  });

  it("preserves the token on 429 so verification can be retried", async () => {
    session.set("db-auditor:session", "valid-token");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response("", { status: 429 }))
        .mockResolvedValueOnce(
          Response.json({ user: { username: "admin", role: "operator" } }),
        ),
    );
    const { api } = await import("./api");
    await expect(api.restoreSession()).rejects.toMatchObject({ status: 429 });
    expect(session.get("db-auditor:session")).toBe("valid-token");
    await expect(api.restoreSession()).resolves.toMatchObject({
      username: "admin",
    });
  });

  it("removes an expired token after a 401", async () => {
    session.set("db-auditor:session", "expired-token");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("", { status: 401 })),
    );
    const { api } = await import("./api");
    await expect(api.restoreSession()).resolves.toBeNull();
    expect(session.has("db-auditor:session")).toBe(false);
  });
});
