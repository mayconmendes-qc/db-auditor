/** Labels PT for PostgreSQL relation classes (Sprint 14 US-051). */
export function relationClassLabel(
  relationClass?: string | null,
  relkind?: string | null,
): string {
  const c = (relationClass || "").toLowerCase();
  if (c === "collection") return "Coleção";
  if (c === "partitioned_table") return "Particionada";
  if (c === "partition") return "Partição";
  if (c === "foreign_table") return "Foreign table";
  if (c === "table") return "Tabela";
  if (relkind === "p") return "Particionada";
  if (relkind === "f") return "Foreign table";
  if (relkind === "r") return "Tabela";
  return relationClass || relkind || "—";
}

export function relationClassBadgeClass(relationClass?: string | null): string {
  const c = (relationClass || "").toLowerCase();
  if (c === "partitioned_table")
    return "border-violet-700/60 bg-violet-950/50 text-violet-200";
  if (c === "partition") return "border-sky-700/60 bg-sky-950/50 text-sky-200";
  if (c === "foreign_table")
    return "border-amber-700/60 bg-amber-950/50 text-amber-200";
  return "border-slate-700 bg-slate-900/80 text-slate-300";
}
