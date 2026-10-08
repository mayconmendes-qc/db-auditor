import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { StoragePieChart } from "./StoragePieChart";

describe("StoragePieChart", () => {
  it("renders an untrusted database label as text, not markup or chart CSS", () => {
    const html = renderToStaticMarkup(
      <StoragePieChart
        items={[
          { label: '</style><script>alert("x")</script>', size_bytes: 1024 },
        ]}
      />,
    );
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
    expect(html).not.toContain("--color-script");
  });
});
