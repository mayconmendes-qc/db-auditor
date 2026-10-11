/** Same labels as the Dashboard connection cards. */
export function connectionPresentation(
  input: {
    dsn_configured: boolean;
    reachable: boolean;
  } | null,
): { label: string; tone: "success" | "danger" } {
  if (!input) {
    return { label: "DSN ausente", tone: "danger" };
  }
  if (input.dsn_configured && input.reachable) {
    return { label: "Conectado", tone: "success" };
  }
  if (input.dsn_configured) {
    return { label: "Indisponível", tone: "danger" };
  }
  return { label: "DSN ausente", tone: "danger" };
}
