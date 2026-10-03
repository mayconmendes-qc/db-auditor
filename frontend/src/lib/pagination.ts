import type { PagedResponse } from "../types";

/** Used only after the user explicitly selects Todas; requests remain bounded. */
export async function fetchAllPages<T>(
  load: (offset: number, limit: number) => Promise<PagedResponse<T>>,
): Promise<T[]> {
  const items: T[] = [];
  for (;;) {
    const result = await load(items.length, 500);
    if (result.items.length === 0 && result.page.has_more) {
      throw new Error("O servidor não avançou na paginação.");
    }
    items.push(...result.items);
    if (items.length > 500000)
      throw new Error("A listagem excede 500.000 linhas. Refine os filtros.");
    if (!result.page.has_more) return items;
  }
}
