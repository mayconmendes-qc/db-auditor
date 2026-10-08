import { useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Select,
  Skeleton,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { formatBytes } from "../lib/format";
import { api } from "../services/api";
import type {
  ActionMeasurement,
  AuditRun,
  EnvironmentCapabilities,
  PageMeta,
  QualityIssue,
  QualityScan,
  TrackedAction,
} from "../types";

const statuses = [
  { value: "suggested", label: "Sugerida" },
  { value: "in_review", label: "Em análise" },
  { value: "planned", label: "Planejada" },
  { value: "executed_externally", label: "Executada externamente" },
  { value: "validated", label: "Validada" },
  { value: "discarded", label: "Descartada" },
];
const qualityLabels: Record<string, string> = {
  null: "Valores ausentes",
  duplicate: "Possível duplicidade",
  orphan: "Referência sem origem",
  date_range: "Data fora do intervalo",
  distribution: "Concentração de valores",
};
const capabilityLabels: Record<string, string> = {
  catalog: "Catálogo",
  constraints: "Restrições",
  indexes: "Índices",
  workload: "Carga de trabalho",
  security: "Segurança",
  timescale: "TimescaleDB",
  data_quality: "Qualidade de dados",
};
function statusLabel(value: string): string {
  return statuses.find((item) => item.value === value)?.label ?? value;
}
function metricLabel(value: string): string {
  switch (value) {
    case "finding_observed":
      return "Achado observado";
    case "table_size_bytes":
      return "Tamanho da tabela";
    case "query_mean_latency_us":
      return "Latência média da consulta (µs)";
    case "query_reads_per_1000_calls":
      return "Blocos lidos por 1.000 chamadas";
    default:
      return value;
  }
}

function words(raw: string): string[] {
  return raw
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean);
}

export function AssistedActionsPage() {
  const { environmentId, environments, setEnvironmentId, openFinding } =
    useApp();
  const [capabilities, setCapabilities] =
    useState<EnvironmentCapabilities | null>(null);
  const [scans, setScans] = useState<QualityScan[]>([]);
  const [actions, setActions] = useState<TrackedAction[]>([]);
  const [actionOffset, setActionOffset] = useState(0);
  const [actionPage, setActionPage] = useState<PageMeta | null>(null);
  const [runs, setRuns] = useState<AuditRun[]>([]);
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [database, setDatabase] = useState("");
  const [schema, setSchema] = useState("public");
  const [table, setTable] = useState("");
  const [limit, setLimit] = useState(500);
  const [nonNull, setNonNull] = useState("");
  const [keys, setKeys] = useState("");
  const [dateColumn, setDateColumn] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [selectedIssue, setSelectedIssue] = useState<QualityIssue | null>(null);
  const [issueStatus, setIssueStatus] = useState("suggested");
  const [owner, setOwner] = useState("");
  const [justification, setJustification] = useState("");
  const [result, setResult] = useState("");
  const [selectedAction, setSelectedAction] = useState<TrackedAction | null>(
    null,
  );
  const [measurements, setMeasurements] = useState<ActionMeasurement[]>([]);
  const [measurementOffset, setMeasurementOffset] = useState(0);
  const [measurementPage, setMeasurementPage] = useState<PageMeta | null>(null);
  const [before, setBefore] = useState("");
  const [after, setAfter] = useState("");
  const [metric, setMetric] =
    useState<ActionMeasurement["metric"]>("finding_observed");
  const [hypothesis, setHypothesis] = useState("");
  const [windowNote, setWindowNote] = useState("");
  const [workloadComparable, setWorkloadComparable] = useState(false);
  const refresh = async (env: string) => {
    setLoading(true);
    setError("");
    try {
      const [c, q, a, r] = await Promise.all([
        api.environmentCapabilities(env),
        api.qualityScans(env),
        api.trackedActions(env, actionOffset),
        api.auditRuns({ environment_id: env }),
      ]);
      setCapabilities(c);
      setScans(q.items);
      setActions(a.items);
      setActionPage(a.page);
      setRuns(
        r.items.filter(
          (run) => run.status === "success" || run.status === "partial_success",
        ),
      );
      setEnabled(q.enabled);
    } catch (cause) {
      setError(
        formatError(cause, "Não foi possível carregar as ações assistidas"),
      );
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    if (environmentId) void refresh(environmentId);
    else {
      setCapabilities(null);
      setScans([]);
      setActions([]);
      setRuns([]);
    }
  }, [environmentId, actionOffset]);
  const startScan = async () => {
    if (!environmentId) return;
    setBusy(true);
    setError("");
    try {
      await api.runQualityScan(environmentId, {
        database: database.trim(),
        schema: schema.trim(),
        table: table.trim(),
        limit,
        expected_non_null: words(nonNull),
        candidate_keys: words(keys),
        date_ranges:
          dateColumn.trim() && dateFrom && dateTo
            ? [
                {
                  column: dateColumn.trim(),
                  from: new Date(dateFrom).toISOString(),
                  to: new Date(dateTo).toISOString(),
                },
              ]
            : [],
      });
      await refresh(environmentId);
    } catch (cause) {
      setError(formatError(cause, "Diagnóstico não concluído"));
    } finally {
      setBusy(false);
    }
  };
  const chooseIssue = (item: QualityIssue) => {
    setSelectedIssue(item);
    setIssueStatus(item.status);
    setOwner(item.owner);
    setJustification(item.justification);
    setResult(item.result);
  };
  const saveIssue = async () => {
    if (!environmentId || !selectedIssue) return;
    setBusy(true);
    try {
      await api.updateQualityIssue(environmentId, selectedIssue.id, {
        status: issueStatus,
        owner,
        justification,
        result,
      });
      setSelectedIssue(null);
      await refresh(environmentId);
    } catch (cause) {
      setError(formatError(cause, "Falha ao salvar decisão"));
    } finally {
      setBusy(false);
    }
  };
  const chooseAction = async (item: TrackedAction) => {
    setSelectedAction(item);
    setMeasurementOffset(0);
    const page = await api.actionMeasurements(item.finding_id);
    setMeasurements(page.items);
    setMeasurementPage(page.page);
    setBefore("");
    setAfter("");
  };
  const measure = async () => {
    if (!selectedAction) return;
    setBusy(true);
    try {
      await api.recordActionMeasurement(selectedAction.finding_id, {
        before_run_id: before,
        after_run_id: after,
        metric,
        hypothesis: hypothesis.trim(),
        window_note: windowNote.trim(),
        workload_comparable: workloadComparable,
      });
      const page = await api.actionMeasurements(selectedAction.finding_id);
      setMeasurements(page.items);
      setMeasurementOffset(0);
      setMeasurementPage(page.page);
      await refresh(environmentId || "");
    } catch (cause) {
      setError(formatError(cause, "Medição não registrada"));
    } finally {
      setBusy(false);
    }
  };
  const exportActions = (format: "csv" | "jsonl") => {
    if (!environmentId) return;
    const link = document.createElement("a");
    link.href = api.actionExportURL(environmentId, format);
    link.download = `acoes-${new Date().toISOString().slice(0, 10)}.${format}`;
    document.body.appendChild(link);
    link.click();
    link.remove();
  };
  return (
    <>
      <PageHeader
        eyebrow="Decisões"
        title="Ações assistidas"
        description="Investigue qualidade de dados somente com autorização. O auditor guarda contagens e decisões; correções ocorrem fora dele, após revisão humana."
      />
      <div className="mt-8 space-y-5">
        <Select
          label="Ambiente"
          value={environmentId ?? ""}
          onChange={(event) => {
            setActionOffset(0);
            setEnvironmentId(event.target.value || null);
          }}
          options={[
            { value: "", label: "Selecione um ambiente" },
            ...environments.map((env) => ({ value: env.id, label: env.name })),
          ]}
        />
        {error ? (
          <ErrorBanner
            message={error}
            onRetry={() => environmentId && void refresh(environmentId)}
          />
        ) : null}
        {loading ? <Skeleton className="h-40" /> : null}
        {!environmentId ? (
          <EmptyState
            title="Selecione um ambiente"
            description="Escolha onde acompanhar ações e diagnósticos."
          />
        ) : null}
        {environmentId && !loading ? (
          <>
            <Card
              title="Capacidades do mecanismo"
              subtitle={`Mecanismo: ${capabilities?.engine ?? "desconhecido"}. Uma capacidade ausente aparece como não aplicável, não como ausência de problema.`}
            >
              <ul className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
                {capabilities?.items.map((item) => (
                  <li
                    key={item.name}
                    className="rounded border border-slate-700 p-2"
                  >
                    {capabilityLabels[item.name] ?? item.name}:{" "}
                    {item.applicable
                      ? "disponível"
                      : `não aplicável — ${item.reason}`}
                  </li>
                ))}
              </ul>
            </Card>
            <Card
              title="Diagnóstico opcional de dados"
              subtitle="Até 1.000 linhas por tabela. Tabelas maiores usam páginas selecionadas pelo PostgreSQL; a margem estatística não é calculável. Nenhum valor de linha é armazenado."
            >
              {!enabled ? (
                <p className="mt-3 text-sm text-amber-300">
                  Desativado por padrão. Um operador deve habilitar
                  AUDITOR_DATA_QUALITY_ENABLED=1 e fornecer uma conta de leitura
                  sem privilégios de escrita.
                </p>
              ) : (
                <div className="mt-3 space-y-3 text-sm">
                  <div className="grid gap-2 sm:grid-cols-3">
                    <input
                      aria-label="Banco"
                      placeholder="Banco"
                      value={database}
                      onChange={(e) => setDatabase(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                    <input
                      aria-label="Schema"
                      placeholder="Schema"
                      value={schema}
                      onChange={(e) => setSchema(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                    <input
                      aria-label="Tabela"
                      placeholder="Tabela"
                      value={table}
                      onChange={(e) => setTable(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                  </div>
                  <label className="block">
                    Limite de linhas{" "}
                    <input
                      type="number"
                      min={1}
                      max={1000}
                      value={limit}
                      onChange={(e) => setLimit(Number(e.target.value))}
                      className="ml-2 w-24 rounded border border-slate-600 bg-slate-900 p-2"
                    />
                  </label>
                  <input
                    aria-label="Colunas obrigatórias, separadas por vírgula"
                    placeholder="Colunas obrigatórias (vírgula)"
                    value={nonNull}
                    onChange={(e) => setNonNull(e.target.value)}
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <input
                    aria-label="Chaves candidatas, separadas por vírgula"
                    placeholder="Chaves candidatas (vírgula)"
                    value={keys}
                    onChange={(e) => setKeys(e.target.value)}
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <div className="grid gap-2 sm:grid-cols-3">
                    <input
                      aria-label="Coluna de data opcional"
                      placeholder="Coluna de data (opcional)"
                      value={dateColumn}
                      onChange={(e) => setDateColumn(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                    <input
                      aria-label="Data inicial"
                      type="datetime-local"
                      value={dateFrom}
                      onChange={(e) => setDateFrom(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                    <input
                      aria-label="Data final"
                      type="datetime-local"
                      value={dateTo}
                      onChange={(e) => setDateTo(e.target.value)}
                      className="rounded border border-slate-600 bg-slate-900 p-2"
                    />
                  </div>
                  <p className="text-slate-400">
                    Órfãos são verificados em chaves estrangeiras simples
                    visíveis ao papel de leitura. Resultados de amostra são
                    hipóteses para confirmação na população.
                  </p>
                  {api.hasRole("auditor") ? (
                    <Button
                      disabled={
                        busy ||
                        !database.trim() ||
                        !schema.trim() ||
                        !table.trim() ||
                        limit < 1 ||
                        limit > 1000 ||
                        (Boolean(dateColumn) && (!dateFrom || !dateTo))
                      }
                      onClick={() => void startScan()}
                    >
                      Executar diagnóstico de leitura
                    </Button>
                  ) : (
                    <p>
                      Seu papel permite consultar resultados; somente auditores
                      podem iniciar diagnóstico.
                    </p>
                  )}
                </div>
              )}
            </Card>
            <Card
              title="Diagnósticos recentes"
              subtitle="Contagens e decisões, sem valores pessoais ou linhas copiadas."
            >
              {scans.length === 0 ? (
                <p className="mt-3 text-sm text-slate-400">
                  Nenhum diagnóstico registrado.
                </p>
              ) : (
                <div className="mt-3 space-y-4">
                  {scans.map((scan) => (
                    <div
                      key={scan.id}
                      className="rounded border border-slate-700 p-3 text-sm"
                    >
                      <p className="font-medium">
                        {scan.database}.{scan.schema}.{scan.table} ·{" "}
                        {new Date(scan.created_at).toLocaleString("pt-BR")}
                      </p>
                      <p className="text-slate-400">
                        {scan.sampled_rows}/{scan.sample_limit} linhas · amostra
                        {scan.sample_method === "paginas_aleatorias_sistema"
                          ? "de páginas selecionadas pelo PostgreSQL"
                          : "limitada por ordem física"}
                        ; mudanças na amostra podem afetar as contagens.
                      </p>
                      <p className="text-slate-400">{scan.sampling_note}</p>
                      <ul className="mt-2 space-y-2">
                        {scan.issues.map((issue) => (
                          <li
                            key={issue.id}
                            className="rounded bg-slate-800/60 p-2"
                          >
                            <p>
                              {qualityLabels[issue.check_kind] ??
                                issue.check_kind}{" "}
                              · {issue.column_name}: {issue.affected_rows}/
                              {issue.sampled_rows} · {statusLabel(issue.status)}
                            </p>
                            <p className="text-slate-300">
                              {issue.plan.meaning}
                            </p>
                            <p className="text-slate-400">
                              {issue.comparison_note}
                              {issue.comparable &&
                              issue.previous_affected_rows != null
                                ? ` Antes: ${issue.previous_affected_rows}; agora: ${issue.affected_rows}.`
                                : ""}
                            </p>
                            {issue.validation_scan_id ? (
                              <p className="text-slate-400">
                                Validação vinculada ao diagnóstico{" "}
                                {issue.validation_scan_id}.
                              </p>
                            ) : null}
                            <details className="mt-1">
                              <summary className="cursor-pointer">
                                Roteiro de confirmação e sanitização
                              </summary>
                              <p>Confirmar: {issue.plan.confirmation}</p>
                              <p>
                                Etapas externas: {issue.plan.external_steps}
                              </p>
                              <p>Validar: {issue.plan.validation}</p>
                              <p>Risco: {issue.plan.risk}</p>
                            </details>
                            {api.hasRole("auditor") ? (
                              <Button
                                variant="secondary"
                                onClick={() => chooseIssue(issue)}
                              >
                                Registrar decisão
                              </Button>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </Card>
            {selectedIssue ? (
              <Card
                title={`Decisão sobre ${qualityLabels[selectedIssue.check_kind] ?? selectedIssue.check_kind} em ${selectedIssue.column_name}`}
                subtitle="Registre responsável e evidência. Nenhuma correção é executada aqui."
              >
                <div className="mt-3 space-y-2">
                  <Select
                    label="Estado"
                    value={issueStatus}
                    onChange={(e) => setIssueStatus(e.target.value)}
                    options={statuses}
                  />
                  {issueStatus === "validated" ? (
                    <p className="text-xs text-amber-200">
                      Para validar, repita o diagnóstico após a ação com método,
                      limite e quantidade de linhas comparáveis. A nova coleta
                      ficará vinculada à decisão.
                    </p>
                  ) : null}
                  <input
                    aria-label="Responsável"
                    value={owner}
                    onChange={(e) => setOwner(e.target.value)}
                    placeholder="Responsável"
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <textarea
                    aria-label="Justificativa"
                    value={justification}
                    onChange={(e) => setJustification(e.target.value)}
                    placeholder="Justificativa"
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <textarea
                    aria-label="Resultado ou evidência externa"
                    value={result}
                    onChange={(e) => setResult(e.target.value)}
                    placeholder="Resultado ou evidência externa"
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <Button disabled={busy} onClick={() => void saveIssue()}>
                    Salvar decisão
                  </Button>
                </div>
              </Card>
            ) : null}
            <Card
              title="Ações de achados"
              subtitle="Acompanhe responsável, prazo, recorrência e observações antes/depois. Correlação não prova causalidade."
            >
              {actions.length ? (
                <>
                  <div className="mt-3">
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        onClick={() => exportActions("jsonl")}
                      >
                        Baixar histórico JSONL
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => exportActions("csv")}
                      >
                        Baixar histórico CSV
                      </Button>
                    </div>
                  </div>
                  <ul className="mt-3 space-y-2">
                    {actions.map((item) => (
                      <li
                        key={item.finding_id}
                        className="rounded border border-slate-700 p-3 text-sm"
                      >
                        <p className="font-medium">
                          {item.title} · {statusLabel(item.status)}
                        </p>
                        <p>
                          Responsável: {item.owner || "não definido"} ·
                          recorrências: {item.recurrences} · prazo:{" "}
                          {item.due_at
                            ? new Date(item.due_at).toLocaleDateString("pt-BR")
                            : "não definido"}
                        </p>
                        {item.latest_measurement ? (
                          <p className="text-slate-400">
                            {metricLabel(item.latest_measurement.metric)}:{" "}
                            {item.latest_measurement.before_value ?? "sem dado"}{" "}
                            →{" "}
                            {item.latest_measurement.after_value ?? "sem dado"}{" "}
                            · {item.latest_measurement.comparison_note}
                          </p>
                        ) : null}
                        <p className="text-slate-400">
                          {item.potential_reclaim_bytes != null
                            ? `Indicador de espaço: ${formatBytes(item.potential_reclaim_bytes)}. `
                            : ""}
                          {item.estimate_note}
                        </p>
                        <div className="mt-2 flex gap-2">
                          <Button
                            variant="secondary"
                            onClick={() => openFinding(item.finding_id)}
                          >
                            Abrir achado
                          </Button>
                          <Button
                            variant="secondary"
                            onClick={() => void chooseAction(item)}
                          >
                            Medir resultado
                          </Button>
                        </div>
                      </li>
                    ))}
                  </ul>
                  {actionPage ? (
                    <div className="mt-3 flex items-center gap-3 text-xs text-slate-300">
                      <Button
                        variant="secondary"
                        disabled={actionOffset === 0}
                        onClick={() =>
                          setActionOffset(Math.max(0, actionOffset - 50))
                        }
                      >
                        Anterior
                      </Button>
                      <span>
                        {actionOffset + 1}–
                        {Math.min(actionOffset + 50, actionPage.total)} de{" "}
                        {actionPage.total} ações
                      </span>
                      <Button
                        variant="secondary"
                        disabled={!actionPage.has_more}
                        onClick={() => setActionOffset(actionOffset + 50)}
                      >
                        Próxima
                      </Button>
                    </div>
                  ) : null}
                </>
              ) : (
                <p className="mt-3 text-sm text-slate-400">
                  Nenhuma ação planejada ou registrada neste ambiente.
                </p>
              )}
            </Card>
            {selectedAction ? (
              <Card
                title={`Medir ação: ${selectedAction.title}`}
                subtitle="Escolha dois runs da mesma cobertura, perfil e versão; informe a hipótese e a janela de observação."
              >
                <div className="mt-3 space-y-2">
                  <Select
                    label="Antes"
                    value={before}
                    onChange={(e) => setBefore(e.target.value)}
                    options={[
                      { value: "", label: "Selecione" },
                      ...runs.map((run) => ({
                        value: run.id,
                        label: `${new Date(run.started_at).toLocaleString("pt-BR")} · ${run.id.slice(0, 8)}`,
                      })),
                    ]}
                  />
                  <Select
                    label="Depois"
                    value={after}
                    onChange={(e) => setAfter(e.target.value)}
                    options={[
                      { value: "", label: "Selecione" },
                      ...runs.map((run) => ({
                        value: run.id,
                        label: `${new Date(run.started_at).toLocaleString("pt-BR")} · ${run.id.slice(0, 8)}`,
                      })),
                    ]}
                  />
                  <Select
                    label="Métrica"
                    value={metric}
                    onChange={(e) =>
                      setMetric(e.target.value as ActionMeasurement["metric"])
                    }
                    options={[
                      {
                        value: "finding_observed",
                        label: "Achado observado (0/1)",
                      },
                      {
                        value: "table_size_bytes",
                        label: "Tamanho da tabela (bytes)",
                      },
                      {
                        value: "query_mean_latency_us",
                        label: "Latência média da consulta (µs)",
                      },
                      {
                        value: "query_reads_per_1000_calls",
                        label: "Blocos lidos por 1.000 chamadas",
                      },
                    ]}
                  />
                  <textarea
                    aria-label="Hipótese"
                    placeholder="Hipótese de melhora, sem afirmar causalidade"
                    value={hypothesis}
                    onChange={(e) => setHypothesis(e.target.value)}
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <textarea
                    aria-label="Janela e carga"
                    placeholder="Janela, carga e mudanças concorrentes"
                    value={windowNote}
                    onChange={(e) => setWindowNote(e.target.value)}
                    className="w-full rounded border border-slate-600 bg-slate-900 p-2"
                  />
                  <label className="flex items-start gap-2 text-sm text-slate-300">
                    <input
                      type="checkbox"
                      checked={workloadComparable}
                      onChange={(event) =>
                        setWorkloadComparable(event.target.checked)
                      }
                    />
                    Confirmo que as cargas de trabalho nas duas janelas são
                    suficientemente semelhantes, com base em evidência
                    registrada na nota acima.
                  </label>
                  {api.hasRole("auditor") ? (
                    <Button
                      disabled={
                        busy ||
                        !before ||
                        !after ||
                        hypothesis.trim().length < 8 ||
                        windowNote.trim().length < 4
                      }
                      onClick={() => void measure()}
                    >
                      Registrar medição
                    </Button>
                  ) : null}
                  <p className="text-xs text-slate-400">
                    Histórico de medições. A carga informada pelo operador é uma
                    ressalva, não uma confirmação automática de condições
                    iguais.
                  </p>
                  <ul
                    className="space-y-2 text-sm"
                    aria-label="Série de medições da ação"
                  >
                    {measurements.map((item) => (
                      <li
                        key={item.id}
                        className="rounded border border-slate-700 p-2"
                      >
                        {new Date(item.recorded_at).toLocaleString("pt-BR")} ·{" "}
                        {metricLabel(item.metric)}:{" "}
                        {item.before_value ?? "sem dado"} →{" "}
                        {item.after_value ?? "sem dado"}. {item.comparison_note}{" "}
                        Hipótese: {item.hypothesis}. Janela e carga:{" "}
                        {item.window_note}.
                      </li>
                    ))}
                  </ul>
                  {measurementPage && measurementPage.total > 50 ? (
                    <div className="mt-3 flex items-center gap-3 text-xs text-slate-300">
                      <Button
                        variant="secondary"
                        disabled={measurementOffset === 0}
                        onClick={() => {
                          const next = Math.max(0, measurementOffset - 50);
                          void api
                            .actionMeasurements(selectedAction.finding_id, next)
                            .then((page) => {
                              setMeasurements(page.items);
                              setMeasurementPage(page.page);
                              setMeasurementOffset(next);
                            })
                            .catch((cause) =>
                              setError(
                                formatError(
                                  cause,
                                  "Falha ao carregar medições",
                                ),
                              ),
                            );
                        }}
                      >
                        Anterior
                      </Button>
                      <span>
                        {measurementOffset + 1}–
                        {Math.min(
                          measurementOffset + 50,
                          measurementPage.total,
                        )}{" "}
                        de {measurementPage.total} medições
                      </span>
                      <Button
                        variant="secondary"
                        disabled={!measurementPage.has_more}
                        onClick={() => {
                          const next = measurementOffset + 50;
                          void api
                            .actionMeasurements(selectedAction.finding_id, next)
                            .then((page) => {
                              setMeasurements(page.items);
                              setMeasurementPage(page.page);
                              setMeasurementOffset(next);
                            })
                            .catch((cause) =>
                              setError(
                                formatError(
                                  cause,
                                  "Falha ao carregar medições",
                                ),
                              ),
                            );
                        }}
                      >
                        Próxima
                      </Button>
                    </div>
                  ) : null}
                </div>
              </Card>
            ) : null}
          </>
        ) : null}
      </div>
    </>
  );
}
