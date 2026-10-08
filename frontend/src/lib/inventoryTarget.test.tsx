import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
  InventoryDetailAvailability,
  InventoryDetailTabs,
} from "../components/InventoryDetailTabs";
import {
  formatLocation,
  type LocationState,
  parseLocationHash,
} from "../context/AppContext";
import {
  type InventoryTarget,
  inventoryPermalink,
  inventoryRunForSelection,
  inventoryTargetKey,
} from "./inventoryTarget";

function location(
  inventory: InventoryTarget | null,
  run = "run-1",
): LocationState {
  return {
    section: "Inventário",
    env: "env-1",
    runId: null,
    findingId: null,
    inventory,
    search: run ? { run } : {},
  };
}

describe("inventory object links", () => {
  const cases: InventoryTarget[] = [
    { kind: "tables", database: "db", schema: "public", name: "orders" },
    { kind: "hypertables", database: "db", schema: "public", name: "metrics" },
    { kind: "indexes", database: "db", schema: "public", name: "orders_pkey" },
    { kind: "views", database: "db", schema: "public", name: "active_orders" },
    {
      kind: "functions",
      database: "db",
      schema: "public",
      name: "total",
      signature: "integer",
    },
    { kind: "caggs", database: "db", schema: "public", name: "metrics_hourly" },
  ];

  it.each(cases)(
    "round-trips $kind without losing run or environment",
    (target) => {
      const hash = formatLocation(location(target));
      const parsed = parseLocationHash(hash);
      expect(parsed.inventory).toEqual(target);
      expect(parsed.env).toBe("env-1");
      expect(parsed.search.run).toBe("run-1");
      expect(formatLocation(parsed)).toBe(hash);
    },
  );

  it("keeps overloaded functions and object types distinct", () => {
    const first = cases[4];
    expect(inventoryTargetKey("env", "run", first)).not.toBe(
      inventoryTargetKey("env", "run", { ...first, signature: "text" }),
    );
    expect(inventoryTargetKey("env", "run", cases[3])).not.toBe(
      inventoryTargetKey("env", "run", { ...cases[3], kind: "caggs" }),
    );
    expect(inventoryTargetKey("env", "run", first)).not.toBe(
      inventoryTargetKey("other-env", "run", first),
    );
  });

  it("keeps a requested historical run when opening another object", () => {
    expect(inventoryRunForSelection("historical", "latest")).toBe("historical");
    expect(inventoryRunForSelection(undefined, "latest")).toBe("latest");
    expect(inventoryRunForSelection(undefined, undefined)).toBeNull();
  });

  it("keeps old table links readable and removes object/run after closing", () => {
    expect(
      parseLocationHash("#/inventory/db/public/orders?env=env-1&run=old")
        .inventory,
    ).toEqual(cases[0]);
    expect(
      parseLocationHash(
        "#/inventory?env=env-1&run=old&database=db&schema=public&table=orders",
      ).inventory,
    ).toEqual(cases[0]);
    const closed = formatLocation(location(null, ""));
    expect(closed).toBe("#/inventory?env=env-1");
    expect(parseLocationHash(closed).inventory).toBeNull();
    expect(parseLocationHash(closed).search.run).toBeUndefined();
  });

  it("encodes names and produces a stable direct link", () => {
    const target: InventoryTarget = {
      kind: "functions",
      database: "sales",
      schema: "public",
      name: "sum total",
      signature: "numeric, text",
    };
    const url = inventoryPermalink(
      "env-1",
      "run-1",
      target,
      "https://example.test/app",
    );
    expect(url).toContain("sum+total");
    expect(
      parseLocationHash(url.split("#")[1] ? `#${url.split("#")[1]}` : "")
        .inventory,
    ).toEqual(target);
  });
});

describe("shared inventory tabs", () => {
  it("renders the six sections with an explicit collection state", () => {
    const markup = renderToStaticMarkup(
      <InventoryDetailTabs tab="overview" onTabChange={() => {}}>
        <InventoryDetailAvailability state="not_collected" />
      </InventoryDetailTabs>,
    );
    expect(markup.match(/role="tab"/g)).toHaveLength(6);
    expect(markup).toContain('aria-selected="true"');
    expect(markup).toContain("Esta informação ainda não é coletada");
    expect(markup).toContain('role="tabpanel"');
  });
});
