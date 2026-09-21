/** Client-side download helpers (no backend dependency). */

function triggerDownload(filename: string, blob: Blob): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function downloadJSON(filename: string, data: unknown): void {
  const body = `${JSON.stringify(data, null, 2)}\n`;
  triggerDownload(
    filename,
    new Blob([body], { type: "application/json;charset=utf-8" }),
  );
}

function escapeCsvCell(value: unknown): string {
  const s = value == null ? "" : String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

/** Build CSV from header keys and row objects (flat string/number fields). */
export function downloadCSV(
  filename: string,
  headers: string[],
  rows: Array<Record<string, unknown>>,
): void {
  const lines = [
    headers.map(escapeCsvCell).join(","),
    ...rows.map((row) => headers.map((h) => escapeCsvCell(row[h])).join(",")),
  ];
  const body = `\uFEFF${lines.join("\n")}\n`;
  triggerDownload(
    filename,
    new Blob([body], { type: "text/csv;charset=utf-8" }),
  );
}
