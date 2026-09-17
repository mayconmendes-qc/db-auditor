import { describe, expect, it } from "vitest";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "success") {
    return "success";
  }
  if (status === "partial_success" || status === "running") {
    return "warning";
  }
  if (status === "failed") {
    return "danger";
  }
  return "neutral";
}

describe("statusTone", () => {
  it("maps statuses", () => {
    expect(statusTone("success")).toBe("success");
    expect(statusTone("failed")).toBe("danger");
    expect(statusTone("partial_success")).toBe("warning");
  });
});
