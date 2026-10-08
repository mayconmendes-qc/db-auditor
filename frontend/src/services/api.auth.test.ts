import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

beforeEach(() => {
  vi.resetModules();
  vi.stubGlobal("window", { dispatchEvent: vi.fn() });
});

afterEach(() => vi.unstubAllGlobals());

describe("cookie session", () => {
  it("logs in without exposing a bearer token and restores after reload", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(
      Response.json({
        user: { username: "admin", role: "operator" },
        csrf_token: "csrf-value",
      }),
    );
    vi.stubGlobal("fetch", fetch);
    const { api } = await import("./api");
    await api.login("admin", "password");
    expect(fetch.mock.calls[0][0]).toContain("/api/v1/auth/login?mode=cookie");
    expect(fetch.mock.calls[0][1].credentials).toBe("include");
    expect(fetch.mock.calls[0][1].headers.Authorization).toBeUndefined();

    vi.resetModules();
    fetch.mockResolvedValueOnce(
      Response.json({
        user: { username: "admin", role: "operator" },
        csrf_token: "csrf-value",
      }),
    );
    const { api: reloaded } = await import("./api");
    await expect(reloaded.restoreSession()).resolves.toEqual({
      username: "admin",
      role: "operator",
    });
    expect(fetch.mock.calls[1][1].credentials).toBe("include");
  });

  it("sends the CSRF token for mutations", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValueOnce(
        Response.json({
          user: { username: "admin", role: "operator" },
          csrf_token: "csrf-value",
        }),
      )
      .mockResolvedValueOnce(Response.json({ status: "logged_out" }));
    vi.stubGlobal("fetch", fetch);
    const { api } = await import("./api");
    await api.login("admin", "password");
    await api.logout();
    expect(fetch.mock.calls[1][1].headers["X-CSRF-Token"]).toBe("csrf-value");
  });

  it("preserves a pending session on 429 so verification can be retried", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response("", { status: 429 }))
        .mockResolvedValueOnce(
          Response.json({
            user: { username: "admin", role: "operator" },
            csrf_token: "csrf-value",
          }),
        ),
    );
    const { api } = await import("./api");
    await expect(api.restoreSession()).rejects.toMatchObject({ status: 429 });
    await expect(api.restoreSession()).resolves.toMatchObject({
      username: "admin",
    });
  });

  it("treats an expired cookie as logged out", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("", { status: 401 })),
    );
    const { api } = await import("./api");
    await expect(api.restoreSession()).resolves.toBeNull();
    expect(api.hasSession()).toBe(false);
  });

  it("revokes the local session in another tab after logout", async () => {
    class FakeChannel {
      static channels: FakeChannel[] = [];
      onmessage: ((event: { data: string }) => void) | null = null;
      constructor(_name: string) {
        FakeChannel.channels.push(this);
      }
      postMessage(data: string) {
        for (const peer of FakeChannel.channels) {
          if (peer !== this) peer.onmessage?.({ data });
        }
      }
    }
    vi.stubGlobal("document", {});
    vi.stubGlobal("BroadcastChannel", FakeChannel);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(async () =>
        Response.json({
          user: { username: "admin", role: "operator" },
          csrf_token: "csrf-value",
        }),
      ),
    );
    const { api: first } = await import("./api");
    await first.login("admin", "password");
    vi.resetModules();
    const { api: second } = await import("./api");
    await second.login("admin", "password");
    expect(second.hasSession()).toBe(true);
    await first.logout();
    expect(second.hasSession()).toBe(false);
  });
});
