import { useMemo, useState } from "react";
import { Badge, Button, Input, Table } from "../../components/ui";
import {
  type PageSize,
  PaginationControls,
} from "../../components/ui/PaginationControls";
import { api } from "../../services/api";
import type { AccessReviewItem } from "../../types";

export interface AccessReviewMatrixProps {
  items: AccessReviewItem[];
  total: number;
  offset: number;
  pageSize: PageSize;
  busy: boolean;
  campaignClosed: boolean;
  onPageSizeChange: (size: PageSize) => void;
  onOffsetChange: (offset: number) => void;
  onDecided: () => void;
  onError: (message: string) => void;
  environmentId: string;
  campaignId: string;
}

function canDecideRow(item: AccessReviewItem): boolean {
  if (!api.hasRole("auditor")) return false;
  const me = api.currentUser();
  if (item.owner_username && item.owner_username === me) return true;
  if (!item.owner_user_id && api.hasRole("operator")) return true;
  return false;
}

export function AccessReviewMatrix({
  items,
  total,
  offset,
  pageSize,
  busy,
  campaignClosed,
  onPageSizeChange,
  onOffsetChange,
  onDecided,
  onError,
  environmentId,
  campaignId,
}: AccessReviewMatrixProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [decision, setDecision] = useState<"approve" | "revoke" | "keep">(
    "approve",
  );
  const [justification, setJustification] = useState("");
  const [revokeDue, setRevokeDue] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const decidable = useMemo(
    () => items.filter((item) => canDecideRow(item) && item.pending),
    [items],
  );

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const submit = async () => {
    if (campaignClosed || selected.size === 0) return;
    if (decision === "keep" && !justification.trim()) {
      onError("Justificativa obrigatória para manter.");
      return;
    }
    setSubmitting(true);
    try {
      const payload = [...selected].map((item_id) => ({
        item_id,
        decision,
        justification: justification.trim(),
        revoke_due_at:
          decision === "revoke" && revokeDue
            ? new Date(revokeDue).toISOString()
            : undefined,
      }));
      await api.postAccessReviewDecisions(environmentId, campaignId, payload);
      setSelected(new Set());
      setJustification("");
      onDecided();
    } catch (err: unknown) {
      onError(
        err instanceof Error ? err.message : "Falha ao registrar decisões.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-3">
      {!campaignClosed && decidable.length > 0 ? (
        <div className="flex flex-wrap items-end gap-3 rounded border border-slate-700 bg-slate-900/60 p-3">
          <label className="text-xs text-slate-300">
            Decisão
            <select
              className="mt-1 block rounded border border-slate-600 bg-slate-950 px-2 py-1 text-sm"
              value={decision}
              onChange={(e) =>
                setDecision(e.target.value as "approve" | "revoke" | "keep")
              }
            >
              <option value="approve">Aprovar</option>
              <option value="revoke">Revogar</option>
              <option value="keep">Manter</option>
            </select>
          </label>
          <Input
            label="Justificativa"
            value={justification}
            onChange={(e) => setJustification(e.target.value)}
            placeholder={
              decision === "keep" ? "Obrigatória para manter" : "Opcional"
            }
          />
          {decision === "revoke" ? (
            <Input
              label="Prazo da revogação"
              type="datetime-local"
              value={revokeDue}
              onChange={(e) => setRevokeDue(e.target.value)}
            />
          ) : null}
          <Button
            disabled={
              submitting ||
              busy ||
              selected.size === 0 ||
              (decision === "keep" && !justification.trim())
            }
            onClick={() => void submit()}
          >
            Aplicar em {selected.size || 0} linha(s)
          </Button>
        </div>
      ) : null}

      <Table
        dense
        pagination={false}
        virtualize
        caption="Matriz de privilégios congelada"
        headers={[
          { id: "sel", label: "Sel." },
          { id: "account", label: "Conta" },
          { id: "object", label: "Objeto" },
          { id: "priv", label: "Privilégio" },
          { id: "origin", label: "Origem" },
          { id: "highlight", label: "Destaque" },
          { id: "lastlogin", label: "Último login" },
          { id: "owner", label: "Responsável" },
          { id: "decision", label: "Decisão" },
        ]}
      >
        {items.map((item) => {
          const obj =
            item.column_name != null && item.column_name !== ""
              ? `${item.object_name}.${item.column_name}`
              : item.object_name;
          const decidableRow =
            canDecideRow(item) && item.pending && !campaignClosed;
          const sampleInactive = (item.highlights ?? []).includes(
            "inactive_or_expired",
          );
          return (
            <tr key={item.id} className="border-t border-slate-800">
              <td className="px-3 py-1.5">
                {decidableRow ? (
                  <input
                    type="checkbox"
                    checked={selected.has(item.id)}
                    onChange={() => toggle(item.id)}
                    aria-label={`Selecionar ${item.account_name} ${item.privilege}`}
                    className="rounded border-slate-600 bg-slate-900 text-emerald-500 focus:ring-emerald-400/50"
                  />
                ) : null}
              </td>
              <td className="px-3 py-1.5 font-mono text-xs">
                {item.account_name}
              </td>
              <td className="px-3 py-1.5 font-mono text-xs">
                {item.object_type}: {obj}
              </td>
              <td className="px-3 py-1.5">{item.privilege}</td>
              <td className="px-3 py-1.5">
                {item.origin_label || item.origin_kind}
              </td>
              <td className="px-3 py-1.5">
                {(item.highlights ?? []).length > 0 ? (
                  <span className="flex flex-wrap gap-1">
                    {item.highlights.map((h) => (
                      <Badge key={h} tone="warning">
                        {h === "inactive_or_expired"
                          ? "sem sessão (amostra)"
                          : h}
                      </Badge>
                    ))}
                  </span>
                ) : (
                  "—"
                )}
              </td>
              <td className="px-3 py-1.5 text-xs">
                {item.last_login_at ? (
                  <span title={`fonte: ${item.last_login_source || "log"}`}>
                    {new Date(item.last_login_at).toLocaleString()}{" "}
                    <Badge tone="neutral">log</Badge>
                  </span>
                ) : sampleInactive ? (
                  <span className="text-slate-400">sem sessão (amostra)</span>
                ) : (
                  "—"
                )}
              </td>
              <td className="px-3 py-1.5">
                {item.owner_username || "operator"}
              </td>
              <td className="px-3 py-1.5">
                {item.pending ? (
                  <Badge tone="neutral">pendente</Badge>
                ) : (
                  <Badge tone="success">{item.decision}</Badge>
                )}
              </td>
            </tr>
          );
        })}
      </Table>
      <PaginationControls
        total={total}
        offset={offset}
        size={pageSize}
        onSizeChange={onPageSizeChange}
        onOffsetChange={onOffsetChange}
        label="Matriz"
        allowAll={false}
      />
    </div>
  );
}
