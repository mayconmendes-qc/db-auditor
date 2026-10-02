import { describe, expect, it } from "vitest";
import { fetchAllPages } from "./pagination";

describe("fetchAllPages", () => {
  it("collects all bounded batches in order", async () => {
    const calls: number[] = [];
    const rows = await fetchAllPages(async (offset) => {
      calls.push(offset);
      return {
        items: offset === 0 ? [1, 2] : [3],
        page: { total: 3, limit: 500, offset, has_more: offset === 0 },
      };
    });
    expect(rows).toEqual([1, 2, 3]);
    expect(calls).toEqual([0, 2]);
  });
  it("rejects a non-advancing server", async () => {
    await expect(
      fetchAllPages(async () => ({
        items: [],
        page: { total: 1, limit: 500, offset: 0, has_more: true },
      })),
    ).rejects.toThrow("avançou");
  });
  it("never returns an incomplete result after a later batch fails", async () => {
    await expect(
      fetchAllPages(async (offset) => {
        if (offset > 0) throw new Error("segundo lote indisponível");
        return {
          items: [1, 2],
          page: { total: 4, limit: 500, offset: 0, has_more: true },
        };
      }),
    ).rejects.toThrow("segundo lote indisponível");
  });
});
