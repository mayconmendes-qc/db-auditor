import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Table } from "./Table";

describe("Table pagination", () => {
  it("shows 20 of 25 rows and all page-size options by default", () => {
    const html = renderToStaticMarkup(
      <Table headers={["Nome"]}>
        {Array.from({ length: 25 }, (_, index) => (
          <tr key={index}>
            <td>Linha {index + 1}</td>
          </tr>
        ))}
      </Table>,
    );
    expect(html).toContain("1–20 de 25");
    expect(html).toContain("20");
    expect(html).toContain("50");
    expect(html).toContain("100");
    expect(html).toContain("Todas");
    expect(html).toContain("Linha 20");
    expect(html).not.toContain("Linha 21");
  });

  it("virtualize mounts a window instead of every row", () => {
    const html = renderToStaticMarkup(
      <Table headers={["Nome"]} pagination={false} virtualize>
        {Array.from({ length: 400 }, (_, index) => (
          <tr key={index}>
            <td>Linha {index + 1}</td>
          </tr>
        ))}
      </Table>,
    );
    expect(html).toContain("Linha 1");
    expect(html).toContain("Linha 30");
    expect(html).not.toContain("Linha 31");
    expect(html).not.toContain("Linha 400");
  });
});
