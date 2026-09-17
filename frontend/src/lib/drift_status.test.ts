import { describe, expect, it } from "vitest";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "MATCH") {
    return "success";
  }
  if (status === "DRIFT" || status === "UNKNOWN") {
    return "warning";
  }
  if (status === "ONLY_SOURCE" || status === "ONLY_TARGET") {
    return "danger";
  }
  return "neutral";
}

describe("drift statusTone", () => {
  it("maps statuses", () => {
    expect(statusTone("MATCH")).toBe("success");
    expect(statusTone("DRIFT")).toBe("warning");
    expect(statusTone("ONLY_SOURCE")).toBe("danger");
  });
});
