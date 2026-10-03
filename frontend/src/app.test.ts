import { describe, expect, it } from "vitest";
import { navigationSections } from "./app";

describe("navigationSections", () => {
  it("provides the auditor navigation in Portuguese", () => {
    expect(navigationSections).toEqual([
      "Dashboard",
      "Documentação",
      "Ambientes",
      "Execuções",
      "Relatórios",
      "Inventário",
      "Mapeamentos",
      "Desvio de schema",
      "Comparar",
      "Findings",
      "Performance",
      "Segurança",
      "Status",
    ]);
  });
});
