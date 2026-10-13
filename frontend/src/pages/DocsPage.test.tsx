import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { DocsPage } from "./DocsPage";
import { docTopics } from "./parts/docsTopics";

describe("DocsPage", () => {
  it("opens topics in place and explains the first one", () => {
    const html = renderToStaticMarkup(<DocsPage />);
    expect(html).toContain("Documentação");
    expect(html).toContain("Por onde começar");
    expect(html).toContain("Inventário");
    expect(html).toContain("Achados");
    expect(html).toContain("Como usar");
    expect(html).toContain("O DB Auditor mostra um retrato do banco");
    expect(html).not.toContain("guia-do-usuario.md");
    expect(html).not.toContain("Carregando o guia");
  });

  it("keeps one title per topic", () => {
    const titles = docTopics.map((topic) => topic.title);
    expect(new Set(titles).size).toBe(titles.length);
    expect(docTopics.length).toBeGreaterThan(10);
    for (const topic of docTopics) {
      expect(topic.intro.length).toBeGreaterThan(40);
      expect(topic.steps.length).toBeGreaterThan(2);
    }
  });
});
