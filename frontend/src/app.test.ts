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
      "Linha do tempo",
      "Comparar",
      "Findings",
      "Digest",
      "Custo",
      "Análises de migration",
      "Revisão de acessos",
      "Contas",
      "Acompanhamento",
      "Ações assistidas",
      "Status",
    ]);
  });
});
