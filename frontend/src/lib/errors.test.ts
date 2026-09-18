import { describe, expect, it } from "vitest";
import { ApiError, formatError, networkApiError } from "./errors";

describe("formatError", () => {
  it("uses ApiError message", () => {
    const err = new ApiError(500, "/x", "Erro interno do servidor. Tente novamente em instantes.");
    expect(formatError(err)).toContain("Erro interno");
  });

  it("falls back for unknown values", () => {
    expect(formatError(null, "padrão")).toBe("padrão");
  });
});

describe("networkApiError", () => {
  it("returns status 0", () => {
    const err = networkApiError("/health");
    expect(err.status).toBe(0);
    expect(err.message).toMatch(/conectar/i);
  });
});
