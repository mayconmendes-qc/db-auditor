import { describe, expect, it } from "vitest";

function statusTone(
  status: string,
): "success" | "warning" | "danger" | "neutral" {
  if (status === "validated") {
    return "success";
  }
  if (status === "suggested" || status === "manual") {
    return "warning";
  }
  if (status === "rejected") {
    return "danger";
  }
  return "neutral";
}

describe("mapping statusTone", () => {
  it("maps statuses", () => {
    expect(statusTone("validated")).toBe("success");
    expect(statusTone("rejected")).toBe("danger");
    expect(statusTone("suggested")).toBe("warning");
  });
});
