/** Portuguese labels for API enum-like values. */

const runStatus: Record<string, string> = {
  running: "Em execução",
  success: "Sucesso",
  partial_success: "Sucesso parcial",
  failed: "Falhou",
  cancelled: "Cancelado",
  skipped: "Ignorado",
};

const findingStatus: Record<string, string> = {
  open: "Aberto",
  acknowledged: "Reconhecido",
  resolved: "Resolvido",
  suppressed: "Suprimido",
};

const severity: Record<string, string> = {
  info: "Informativo",
  low: "Baixa",
  medium: "Média",
  high: "Alta",
  critical: "Crítica",
};

const mappingStatus: Record<string, string> = {
  suggested: "Sugerido",
  validated: "Validado",
  rejected: "Rejeitado",
  manual: "Manual",
};

const envType: Record<string, string> = {
  tiger_cloud: "Tiger Cloud",
  self_hosted: "Self-hosted",
};

const discoveryMode: Record<string, string> = {
  single_database: "Banco único",
  multi_database: "Múltiplos bancos",
};

function mapLabel(table: Record<string, string>, value: string | null | undefined): string {
  if (!value) {
    return "—";
  }
  return table[value] ?? value;
}

export const labels = {
  runStatus: (v: string) => mapLabel(runStatus, v),
  findingStatus: (v: string) => mapLabel(findingStatus, v),
  severity: (v: string) => mapLabel(severity, v),
  mappingStatus: (v: string) => mapLabel(mappingStatus, v),
  envType: (v: string) => mapLabel(envType, v),
  discoveryMode: (v: string) => mapLabel(discoveryMode, v),
};
