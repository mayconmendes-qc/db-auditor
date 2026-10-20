import { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Table,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { formatError } from "../lib/errors";
import { api } from "../services/api";
import type { PiiMapItem } from "../types";

export function PersonalDataPage() {
  const { environmentId } = useApp();
  const [rows, setRows] = useState<PiiMapItem[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [applicable, setApplicable] = useState(true);
  const [reason, setReason] = useState("");
  const [contentEnabled, setContentEnabled] = useState(false);
  const [category, setCategory] = useState("");
  const [confidence, setConfidence] = useState("");
  const [withContent, setWithContent] = useState(false);
  const canAudit = api.hasRole("auditor");

  const load = useCallback(async () => {
    if (!environmentId) return;
    setBusy(true);
    setError(null);
    try {
      const scans = await api.piiScans(environmentId);
      setApplicable(scans.applicable !== false);
      setReason(scans.reason || "");
      setContentEnabled(!!scans.content_scan_enabled);
      if (scans.applicable === false) {
        setRows([]);
        setTotal(0);
        return;
      }
      const map = await api.piiMap(environmentId, {
        category: category || undefined,
        confidence: confidence || undefined,
        limit: 50,
        offset: 0,
      });
      setRows(map.items || []);
      setTotal(map.total ?? (map.items || []).length);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao carregar mapa de dados pessoais"));
    } finally {
      setBusy(false);
    }
  }, [environmentId, category, confidence]);

  useEffect(() => {
    void load();
  }, [load]);

  const startScan = async () => {
    if (!environmentId || !canAudit) return;
    setBusy(true);
    setError(null);
    try {
      const stages = withContent ? [1, 2] : [1];
      await api.startPiiScan(environmentId, { stages });
      await load();
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao iniciar varredura"));
    } finally {
      setBusy(false);
    }
  };

  const review = async (
    qualified: string,
    decision: "confirmed" | "discarded",
  ) => {
    if (!environmentId || !canAudit) return;
    let justification = "";
    if (decision === "discarded") {
      justification =
        window.prompt("Justificativa do descarte (obrigatória):") || "";
      if (!justification.trim()) return;
    }
    try {
      await api.reviewPiiClassification(environmentId, qualified, {
        decision,
        justification,
      });
      await load();
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao registrar revisão"));
    }
  };

  const exportCsv = async () => {
    if (!environmentId || !canAudit) return;
    try {
      await api.exportPiiMapCsv(environmentId);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao exportar CSV"));
    }
  };

  if (!environmentId) {
    return (
      <EmptyState
        title="Selecione um ambiente"
        description="Escolha um ambiente no seletor superior."
      />
    );
  }

  if (!applicable) {
    return (
      <>
        <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
          DADOS PESSOAIS
        </p>
        <h1 className="mt-2 text-3xl font-semibold text-slate-50">Mapa LGPD</h1>
        <Card className="mt-6">
          <p role="status">{reason || "não aplicável a este mecanismo"}</p>
        </Card>
      </>
    );
  }

  return (
    <>
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        DADOS PESSOAIS
      </p>
      <h1 className="mt-2 text-3xl font-semibold text-slate-50 md:text-4xl">
        Mapa de dados pessoais (LGPD)
      </h1>
      <p className="mt-2 max-w-3xl text-slate-300">
        Classificação por nome/tipo e, opcionalmente, validação de conteúdo em
        memória. Nenhum valor amostrado é exibido ou exportado.
      </p>

      {error ? (
        <div className="mt-4">
          <ErrorBanner message={error} />
        </div>
      ) : null}

      <Card className="mt-6 flex flex-wrap items-end gap-3">
        <label className="text-sm text-slate-300">
          Categoria
          <select
            className="mt-1 block rounded border border-slate-700 bg-slate-900 px-2 py-1"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
          >
            <option value="">Todas</option>
            {[
              "cpf",
              "cnpj",
              "email",
              "telefone",
              "cep",
              "pix",
              "nome",
              "nascimento",
              "endereco",
              "rg",
            ].map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-slate-300">
          Confiança
          <select
            className="mt-1 block rounded border border-slate-700 bg-slate-900 px-2 py-1"
            value={confidence}
            onChange={(e) => setConfidence(e.target.value)}
          >
            <option value="">Todas</option>
            <option value="alta">alta</option>
            <option value="media">média</option>
            <option value="baixa">baixa</option>
          </select>
        </label>
        {canAudit ? (
          <>
            <label className="flex items-center gap-2 text-sm text-slate-300">
              <input
                type="checkbox"
                checked={withContent}
                disabled={!contentEnabled}
                onChange={(e) => setWithContent(e.target.checked)}
              />
              Validar conteúdo
              {!contentEnabled ? (
                <span className="text-amber-300">(etapa 2 desligada)</span>
              ) : null}
            </label>
            <Button
              type="button"
              onClick={() => void startScan()}
              disabled={busy}
            >
              Iniciar varredura
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => void exportCsv()}
            >
              Exportar CSV
            </Button>
          </>
        ) : null}
      </Card>

      <Card className="mt-4">
        {busy && rows.length === 0 ? <p role="status">Carregando…</p> : null}
        {!busy && rows.length === 0 ? (
          <EmptyState
            title="Nenhuma coluna classificada"
            description="Execute uma varredura (etapa 1) após uma coleta completa."
          />
        ) : null}
        {rows.length > 0 ? (
          <Table
            headers={[
              "Coluna",
              "Categoria",
              "Confiança",
              "Método",
              "% validado",
              "Leitura",
              "RLS",
              "Revisão",
            ]}
          >
            {rows.map((r) => (
              <tr key={r.id || r.qualified}>
                <td className="font-mono text-xs">{r.qualified}</td>
                <td>{r.category}</td>
                <td>
                  <Badge
                    tone={
                      r.confidence === "alta"
                        ? "danger"
                        : r.confidence === "media"
                          ? "warning"
                          : "neutral"
                    }
                  >
                    {r.confidence}
                  </Badge>
                </td>
                <td>{r.method}</td>
                <td>{r.valid_percent?.toFixed?.(1) ?? r.valid_percent}</td>
                <td>
                  {r.read_accounts} (
                  {r.access_level === "nivel_tabela"
                    ? "nível tabela"
                    : "coluna"}
                  )
                </td>
                <td>{r.has_rls ? "sim" : "não"}</td>
                <td>
                  {r.review_decision || "—"}
                  {canAudit ? (
                    <span className="ml-2 inline-flex gap-1">
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() => void review(r.qualified, "confirmed")}
                      >
                        Confirmar
                      </Button>
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() => void review(r.qualified, "discarded")}
                      >
                        Descartar
                      </Button>
                    </span>
                  ) : null}
                </td>
              </tr>
            ))}
          </Table>
        ) : null}
        <p className="mt-2 text-xs text-slate-400">{total} coluna(s)</p>
      </Card>
    </>
  );
}
