import { describe, expect, it } from "vitest";
import { parseLocationHash } from "./AppContext";

describe("parseLocationHash", () => {
  it("restores a section, environment and finding id the way a reload would", () => {
    const parsed = parseLocationHash(
      "#/findings/11111111-1111-1111-1111-111111111111?env=22222222-2222-2222-2222-222222222222&status=open",
    );
    expect(parsed.section).toBe("Findings");
    expect(parsed.findingId).toBe("11111111-1111-1111-1111-111111111111");
    expect(parsed.env).toBe("22222222-2222-2222-2222-222222222222");
    expect(parsed.search.status).toBe("open");
  });

  it("opens the old performance, security and rules links on Análises", () => {
    expect(parseLocationHash("#/performance").section).toBe("Findings");
    expect(parseLocationHash("#/performance").search.category).toBe(
      "performance",
    );
    const security = parseLocationHash("#/security?env=env-1");
    expect(security.section).toBe("Findings");
    expect(security.env).toBe("env-1");
    expect(security.search.category).toBe("security");
    expect(parseLocationHash("#/rules").search.view).toBe("rules");
  });
});
