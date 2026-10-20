import { Button, Card, Input } from "../../components/ui";
import type { NotificationDestination, QueryDigestConfig } from "../../types";

export function DigestConfigCard({
  config,
  windowDays,
  deliveryEnabled,
  busy,
  onWindowDays,
  onDeliveryEnabled,
  onSave,
}: {
  config: QueryDigestConfig | null;
  windowDays: number;
  deliveryEnabled: boolean;
  busy: boolean;
  onWindowDays: (n: number) => void;
  onDeliveryEnabled: (v: boolean) => void;
  onSave: () => void;
}) {
  return (
    <Card
      title="Configuração"
      subtitle={
        config
          ? `Atualizado por ${config.updated_by || "—"}`
          : "Janela e entrega"
      }
    >
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          label="Janela (dias)"
          type="number"
          min={1}
          max={90}
          value={String(windowDays)}
          onChange={(e) => onWindowDays(Number(e.target.value) || 7)}
        />
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input
            type="checkbox"
            checked={deliveryEnabled}
            onChange={(e) => onDeliveryEnabled(e.target.checked)}
          />
          Entrega ligada
        </label>
      </div>
      <div className="mt-3">
        <Button disabled={busy} onClick={onSave}>
          Salvar configuração
        </Button>
      </div>
    </Card>
  );
}

export function DigestDestinationsCard({
  usingGlobal,
  destinations,
  emailDraft,
  webhookDraft,
  busy,
  onEmailDraft,
  onWebhookDraft,
  onSave,
}: {
  usingGlobal: boolean;
  destinations: NotificationDestination[];
  emailDraft: string;
  webhookDraft: string;
  busy: boolean;
  onEmailDraft: (v: string) => void;
  onWebhookDraft: (v: string) => void;
  onSave: () => void;
}) {
  return (
    <Card
      title="Destinatários do ambiente"
      subtitle={
        usingGlobal
          ? "Usando canais globais (fallback)"
          : "Destinos específicos deste ambiente"
      }
    >
      <p className="text-xs text-slate-400">
        A URL completa do webhook nunca é exibida. Ao salvar, informe de novo os
        destinos desejados (substitui a lista).
      </p>
      <ul className="mt-3 space-y-1 text-sm">
        {destinations.map((d) => (
          <li key={d.id}>
            {d.channel} · {d.target_masked} · {d.format}
            {d.active ? "" : " (inativo)"}
          </li>
        ))}
      </ul>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <Input
          label="E-mail"
          value={emailDraft}
          onChange={(e) => onEmailDraft(e.target.value)}
          placeholder="ops@empresa.example"
        />
        <Input
          label="Webhook https"
          value={webhookDraft}
          onChange={(e) => onWebhookDraft(e.target.value)}
          placeholder="https://hooks.example.com/…"
        />
      </div>
      <div className="mt-3">
        <Button disabled={busy} onClick={onSave}>
          Substituir destinatários
        </Button>
      </div>
    </Card>
  );
}
