import { describe, expect, it } from "vitest";
import type { RunTrendPoint } from "../types";
import { trendEntries } from "./trend-gaps";

function point(environment_id: string, at: string): RunTrendPoint {
  return {
    environment_id,
    environment_name: environment_id,
    audit_run_id: `${environment_id}-${at}`,
    at,
    status: "success",
    profile: "daily",
    collector_version: "1",
    coverage: "complete",
    comparable: true,
    score_confidence: 1,
  };
}

describe("trendEntries", () => {
  it("marks absent UTC days without treating another environment as a measurement", () => {
    const entries = trendEntries(
      [
        point("a", "2026-10-01T23:30:00Z"),
        point("b", "2026-10-02T00:30:00Z"),
        point("a", "2026-10-04T00:30:00Z"),
      ],
      "day",
    );
    expect(entries.filter((entry) => entry.kind === "gap")).toEqual([
      { kind: "gap", key: expect.any(String), environment: "a", periods: 2 },
    ]);
  });

  it("handles month and week boundaries in UTC", () => {
    const points = [
      point("a", "2026-12-31T23:00:00Z"),
      point("a", "2027-02-01T01:00:00Z"),
    ];
    expect(
      trendEntries(points, "month").filter((entry) => entry.kind === "gap"),
    ).toHaveLength(1);
    expect(
      trendEntries(points, "week").filter((entry) => entry.kind === "gap"),
    ).toHaveLength(1);
  });
});
