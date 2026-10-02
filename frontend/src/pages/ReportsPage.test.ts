import { describe, expect, it } from "vitest";
import type { ReportJob } from "../types";
import { reportFormReady, reportJobActions } from "./ReportsPage";

const job: ReportJob = {
  id: "job",
  environment_id: "env",
  audit_run_id: "run",
  report_type: "executive",
  filters: {},
  requested_by: "report-token",
  rule_version: "1",
  status: "queued",
  attempts: 0,
  created_at: "2026-10-02T00:00:00Z",
  expires_at: "2026-11-01T00:00:00Z",
};

describe("PDF report controls", () => {
  it("requires a complete table scope and an in-session token", () => {
    expect(
      reportFormReady("env", "run", "token", "table", {
        database: "db",
        schema: "public",
        table: "orders",
      }),
    ).toBe(true);
    expect(
      reportFormReady("env", "run", "", "table", {
        database: "db",
        schema: "public",
        table: "orders",
      }),
    ).toBe(false);
    expect(
      reportFormReady("env", "run", "token", "table", {
        database: "db",
        schema: "public",
      }),
    ).toBe(false);
    expect(
      reportFormReady("env", "run", "token", "technical", { schema: "public" }),
    ).toBe(false);
  });

  it("offers only actions allowed by the job state and expiration", () => {
    const now = Date.parse("2026-10-03T00:00:00Z");
    expect(reportJobActions(job, now)).toEqual(["cancel"]);
    expect(reportJobActions({ ...job, status: "running" }, now)).toEqual([
      "cancel",
    ]);
    expect(
      reportJobActions({ ...job, status: "failed", attempts: 2 }, now),
    ).toEqual(["retry"]);
    expect(
      reportJobActions({ ...job, status: "failed", attempts: 3 }, now),
    ).toEqual([]);
    expect(reportJobActions({ ...job, status: "success" }, now)).toEqual([
      "download",
    ]);
    expect(
      reportJobActions(
        { ...job, status: "success" },
        Date.parse("2026-12-01T00:00:00Z"),
      ),
    ).toEqual([]);
  });
});
