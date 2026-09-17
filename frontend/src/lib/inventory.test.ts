import { describe, expect, it } from "vitest";

function hypertableKey(h: {
  database_name: string;
  schema_name: string;
  hypertable_name: string;
}) {
  return `${h.database_name}.${h.schema_name}.${h.hypertable_name}`;
}

describe("hypertableKey", () => {
  it("builds stable key", () => {
    expect(
      hypertableKey({
        database_name: "tsdb",
        schema_name: "public",
        hypertable_name: "metrics",
      }),
    ).toBe("tsdb.public.metrics");
  });
});
