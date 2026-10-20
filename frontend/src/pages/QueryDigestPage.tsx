import { useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Select,
  Skeleton,
} from "../components/ui";
import { useApp } from "../context/AppContext";
import { useLatestSignal } from "../hooks/useLatestSignal";
import { formatError, isAbortError } from "../lib/errors";
import { api } from "../services/api";
import type {
  NotificationDestination,
  NotificationDestinationInput,
  QueryDigest,
  QueryDigestConfig,
} from "../types";
import {
  DigestConfigCard,
  DigestDestinationsCard,
} from "./parts/DigestOperatorCards";
import { DigestSection, itemsOf } from "./parts/DigestSection";

export function QueryDigestPage() {
  const { environmentId, environments, setEnvironmentId, openFinding } =
    useApp();
  const isOperator = api.hasRole("operator");
  const [list, setList] = useState<QueryDigest[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [detail, setDetail] = useState<QueryDigest | null>(null);
  const [config, setConfig] = useState<QueryDigestConfig | null>(null);
  const [windowDays, setWindowDays] = useState(7);
  const [deliveryEnabled, setDeliveryEnabled] = useState(true);
  const [destinations, setDestinations] = useState<NotificationDestination[]>(
    [],
  );
  const [usingGlobal, setUsingGlobal] = useState(true);
  const [emailDraft, setEmailDraft] = useState("");
  const [webhookDraft, setWebhookDraft] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const nextSignal = useLatestSignal();

  const refresh = async (env: string) => {
    const signal = nextSignal();
    setLoading(true);
    try {
      const digests = await api.queryDigests(env, { limit: 50 }, signal);
      if (signal.aborted) return;
      setList(digests.items);
      setSelectedId((current) =>
        digests.items.some((d) => d.id === current)
          ? current
          : (digests.items[0]?.id ?? ""),
      );
      const cfg = await api.queryDigestConfig(env, signal);
      if (!signal.aborted) {
        setConfig(cfg);
        setWindowDays(cfg.window_days);
        setDeliveryEnabled(cfg.delivery_enabled);
      }
      if (isOperator) {
        try {
          const dest = await api.notificationDestinations(env, signal);
          if (!signal.aborted) {
            setDestinations(dest.items);
            setUsingGlobal(dest.using_global_fallback);
          }
        } catch {
          if (!signal.aborted) {
            setDestinations([]);
            setUsingGlobal(true);
          }
        }
      } else {
        setDestinations([]);
      }
      if (!signal.aborted) setError(null);
    } catch (cause) {
      if (signal.aborted || isAbortError(cause)) return;
      setError(formatError(cause, "Não foi possível carregar os digests."));
    } finally {
      if (!signal.aborted) setLoading(false);
    }
  };

  useEffect(() => {
    if (environmentId) void refresh(environmentId);
    else {
      nextSignal();
      setList([]);
      setDetail(null);
      setSelectedId("");
      setLoading(false);
    }
  }, [environmentId]);

  useEffect(() => {
    if (!environmentId || !selectedId) {
      setDetail(null);
      return;
    }
    const signal = nextSignal();
    void api
      .queryDigest(environmentId, selectedId, signal)
      .then((value) => {
        if (!signal.aborted) setDetail(value);
      })
      .catch((cause) => {
        if (signal.aborted || isAbortError(cause)) return;
        setError(formatError(cause, "Não foi possível abrir o digest."));
      });
  }, [environmentId, selectedId]);

  const generate = async () => {
    if (!environmentId || !isOperator) return;
    setBusy(true);
    try {
      const created = await api.generateQueryDigest(environmentId);
      setSelectedId(created.id);
      await refresh(environmentId);
    } catch (cause) {
      setError(formatError(cause, "Falha ao gerar digest."));
    } finally {
      setBusy(false);
    }
  };

  const saveConfig = async () => {
    if (!environmentId || !isOperator) return;
    setBusy(true);
    try {
      const saved = await api.updateQueryDigestConfig(environmentId, {
        window_days: windowDays,
        delivery_enabled: deliveryEnabled,
      });
      setConfig(saved);
    } catch (cause) {
      setError(formatError(cause, "Falha ao salvar configuração."));
    } finally {
      setBusy(false);
    }
  };

  const saveDestinations = async () => {
    if (!environmentId || !isOperator) return;
    setBusy(true);
    try {
      const items: NotificationDestinationInput[] = [];
      if (emailDraft.trim()) {
        items.push({
          channel: "email",
          target: emailDraft.trim(),
          format: "generic",
          active: true,
        });
      }
      if (webhookDraft.trim()) {
        items.push({
          channel: "webhook",
          target: webhookDraft.trim(),
          format: "generic",
          active: true,
        });
      }
      const result = await api.replaceNotificationDestinations(
        environmentId,
        items,
      );
      setDestinations(result.items);
      setUsingGlobal(result.using_global_fallback);
      setEmailDraft("");
      setWebhookDraft("");
    } catch (cause) {
      setError(formatError(cause, "Falha ao salvar destinatários."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Análise"
        title="Digest de consultas"
        description="Compara duas janelas de workload e resume regressões, consultas novas, sumidas e melhoras — sem literais SQL."
      />
      <div className="mt-8 space-y-5">
        <Select
          label="Ambiente"
          value={environmentId ?? ""}
          onChange={(e) => setEnvironmentId(e.target.value || null)}
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
        {loading ? <Skeleton className="h-36" /> : null}
        {!environmentId ? (
          <EmptyState
            title="Selecione um ambiente"
            description="Escolha o ambiente para ver o histórico de digests."
          />
        ) : null}
        {environmentId && !loading ? (
          <>
            <Card
              title="Histórico"
              subtitle="Gerado após a coleta que fecha a janela, ou sob demanda."
            >
              {isOperator ? (
                <div className="mb-3">
                  <Button disabled={busy} onClick={() => void generate()}>
                    Gerar agora
                  </Button>
                </div>
              ) : null}
              {list.length === 0 ? (
                <EmptyState
                  className="mt-3"
                  title="Nenhum digest ainda"
                  description="Aguarde o fechamento da janela ou peça a um operador para gerar."
                />
              ) : (
                <Select
                  label="Digest"
                  value={selectedId}
                  onChange={(e) => setSelectedId(e.target.value)}
                  options={list.map((d) => ({
                    value: d.id,
                    label: `${new Date(d.created_at).toLocaleString()} · ${d.state} · R${d.regression_count}/N${d.new_count}`,
                  }))}
                />
              )}
            </Card>

            {detail ? (
              <>
                <Card
                  title="Cobertura e estado"
                  subtitle="Ressalvas honestas do top N e da janela."
                >
                  <div className="mt-1 space-y-2 text-sm text-slate-200">
                    <p className="flex items-center gap-2">
                      Estado:{" "}
                      {detail.state === "ok" ? (
                        <Badge tone="success">ok</Badge>
                      ) : detail.state === "not_applicable" ? (
                        <Badge tone="warning">não aplicável</Badge>
                      ) : (
                        <Badge tone="neutral">sem janela comparável</Badge>
                      )}
                    </p>
                    {detail.reason ? <p>Motivo: {detail.reason}</p> : null}
                    {detail.state === "not_applicable" ? (
                      <p role="status">
                        Não aplicável a este mecanismo (ex.: MongoDB ou ausência
                        de pg_stat_statements).
                      </p>
                    ) : null}
                    <p>Top N coletado: {detail.workload_limit_n}</p>
                    {detail.subwindow_note ? (
                      <p>Subjanela: {detail.subwindow_note}</p>
                    ) : null}
                    {(detail.coverage_notes ?? []).map((note) => (
                      <p key={note}>• {note}</p>
                    ))}
                  </div>
                </Card>
                {detail.state === "ok" ? (
                  <>
                    <DigestSection
                      title="Regressões"
                      rows={itemsOf(detail, "regressao")}
                      onOpenFinding={openFinding}
                    />
                    <DigestSection
                      title="Consultas novas"
                      rows={itemsOf(detail, "nova")}
                      onOpenFinding={openFinding}
                    />
                    <DigestSection
                      title="Consultas que sumiram"
                      rows={itemsOf(detail, "sumiu")}
                      onOpenFinding={openFinding}
                    />
                    <DigestSection
                      title="Melhoras"
                      rows={itemsOf(detail, "melhora")}
                      onOpenFinding={openFinding}
                    />
                  </>
                ) : null}
                <Card
                  title="Entregas"
                  subtitle="Histórico de e-mail e webhook."
                >
                  {(detail.deliveries ?? []).length === 0 ? (
                    <p className="mt-2 text-sm text-slate-400">
                      Nenhuma entrega registrada.
                    </p>
                  ) : (
                    <ul className="mt-3 space-y-2 text-sm">
                      {(detail.deliveries ?? []).map((d) => (
                        <li key={d.id}>
                          <Badge
                            tone={
                              d.status === "sent"
                                ? "success"
                                : d.status === "failed"
                                  ? "danger"
                                  : "neutral"
                            }
                          >
                            {d.status}
                          </Badge>{" "}
                          {d.channel} · {d.target_masked || "—"} · tentativa{" "}
                          {d.attempt}
                          {d.error_sanitized ? ` · ${d.error_sanitized}` : ""}
                        </li>
                      ))}
                    </ul>
                  )}
                </Card>
              </>
            ) : null}

            {isOperator ? (
              <>
                <DigestConfigCard
                  config={config}
                  windowDays={windowDays}
                  deliveryEnabled={deliveryEnabled}
                  busy={busy}
                  onWindowDays={setWindowDays}
                  onDeliveryEnabled={setDeliveryEnabled}
                  onSave={() => void saveConfig()}
                />
                <DigestDestinationsCard
                  usingGlobal={usingGlobal}
                  destinations={destinations}
                  emailDraft={emailDraft}
                  webhookDraft={webhookDraft}
                  busy={busy}
                  onEmailDraft={setEmailDraft}
                  onWebhookDraft={setWebhookDraft}
                  onSave={() => void saveDestinations()}
                />
              </>
            ) : (
              <p className="text-sm text-slate-400">
                Configuração e destinatários são visíveis só para operadores.
              </p>
            )}
          </>
        ) : null}
      </div>
    </>
  );
}
