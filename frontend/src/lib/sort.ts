export type SortDir = "asc" | "desc";

export interface SortState {
  key: string;
  dir: SortDir;
}

export function nextSort(current: SortState | null, key: string): SortState {
  if (current?.key === key) {
    return { key, dir: current.dir === "asc" ? "desc" : "asc" };
  }
  return { key, dir: "asc" };
}

export function compareValues(
  a: string | number | boolean | null | undefined,
  b: string | number | boolean | null | undefined,
  dir: SortDir,
): number {
  const av = a ?? "";
  const bv = b ?? "";
  let cmp = 0;
  if (typeof av === "number" && typeof bv === "number") {
    cmp = av - bv;
  } else if (typeof av === "boolean" && typeof bv === "boolean") {
    cmp = Number(av) - Number(bv);
  } else {
    cmp = String(av).localeCompare(String(bv), "pt", { numeric: true });
  }
  return dir === "asc" ? cmp : -cmp;
}

export function sortBy<T>(
  items: T[],
  sort: SortState | null,
  getters: Record<string, (item: T) => string | number | boolean | null | undefined>,
): T[] {
  if (!sort || !getters[sort.key]) {
    return items;
  }
  const get = getters[sort.key];
  return [...items].sort((a, b) => compareValues(get(a), get(b), sort.dir));
}
