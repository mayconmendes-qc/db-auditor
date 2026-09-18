import { describe, expect, it } from "vitest";
import { navigationSections } from "./app";

describe("navigationSections", () => {
  it("provides the auditor navigation in Portuguese", () => {
    expect(navigationSections).toEqual([
      "Visão geral",
      "Dashboard",
      "Documentação",
      "Ambientes",
      "Execuções",
      "Inventário",
      "Mapeamentos",
      "Desvio de schema",
      "Findings",
      "Performance",
      "Segurança",
      "Status",
    ]);
  });
});
