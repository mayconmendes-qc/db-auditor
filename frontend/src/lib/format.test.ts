import { describe, expect, it } from "vitest";
import { formatBytes, matchesSearch } from "./format";

describe("formatBytes", () => {
  it("formats small values as bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(512)).toBe("512 B");
  });

  it("formats larger values with units", () => {
    expect(formatBytes(2048)).toBe("2 KB");
    expect(formatBytes(5 * 1024 * 1024)).toBe("5.0 MB");
  });

  it("guards invalid input", () => {
    expect(formatBytes(-1)).toBe("0 B");
    expect(formatBytes(Number.NaN)).toBe("0 B");
  });
});

describe("matchesSearch", () => {
  it("matches case-insensitively", () => {
    expect(matchesSearch("Public.Users", "users")).toBe(true);
    expect(matchesSearch("tsdb", "TS")).toBe(true);
  });

  it("returns true for empty query", () => {
    expect(matchesSearch("anything", "")).toBe(true);
    expect(matchesSearch("anything", "   ")).toBe(true);
  });

  it("returns false when there is no match", () => {
    expect(matchesSearch("public", "other")).toBe(false);
  });
});
