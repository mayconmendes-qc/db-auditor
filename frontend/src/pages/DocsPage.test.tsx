import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { DocsPage } from "./DocsPage";

describe("DocsPage", () => {
  it("covers the current sprint workflows and safe migrations", () => {
    const html = renderToStaticMarkup(<DocsPage />);
    for (const term of [
      "Relatórios",
      "Contas e permissões",
      "Total de Databases",
      "cobertura parcial",
      "Configuração inicial (ops)",
    ]) {
      expect(html).toContain(term);
    }
  });
});
