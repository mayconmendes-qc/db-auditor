import { describe, expect, it } from "vitest";
import { connectionPresentation } from "./connectionStatus";

describe("connectionPresentation", () => {
  it("matches the dashboard connection cards", () => {
    expect(
      connectionPresentation({ dsn_configured: true, reachable: true }).label,
    ).toBe("Conectado");
    expect(
      connectionPresentation({ dsn_configured: true, reachable: false }).label,
    ).toBe("Indisponível");
    expect(
      connectionPresentation({ dsn_configured: false, reachable: false }).label,
    ).toBe("DSN ausente");
    expect(connectionPresentation(null).label).toBe("DSN ausente");
  });
});
