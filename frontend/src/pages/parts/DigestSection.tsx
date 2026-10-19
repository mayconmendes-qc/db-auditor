import { Badge, Card, Table } from "../../components/ui";
import type { QueryDigestItem } from "../../types";

export function DigestSection({
  title,
  rows,
  onOpenFinding,
}: {
  title: string;
  rows: QueryDigestItem[];
  onOpenFinding: (id: string) => void;
}) {
  return (
    <Card title={title} subtitle={`${rows.length} item(ns)`}>
      {rows.length === 0 ? (
        <p className="mt-2 text-sm text-slate-400">Nenhum item nesta seção.</p>
      ) : (
        <Table
          className="mt-3"
          dense
          headers={[
            "Fingerprint",
            "Objetos",
            "Chamadas",
            "Média ms",
            "Variação",
            "Links",
          ]}
        >
          {rows.map((item) => (
            <tr key={item.id} className="border-t border-slate-800">
              <td className="px-3 py-2 font-mono text-xs">
                {item.fingerprint.length > 40
                  ? `${item.fingerprint.slice(0, 40)}…`
                  : item.fingerprint}
              </td>
              <td className="px-3 py-2 text-xs">
                {item.referenced_objects.join(", ") || "—"}
              </td>
              <td className="px-3 py-2 text-xs">
                {item.prior_calls}→{item.current_calls}
              </td>
              <td className="px-3 py-2 text-xs">
                {item.prior_mean_ms.toFixed(1)}→
                {item.current_mean_ms.toFixed(1)}
              </td>
              <td className="px-3 py-2 text-xs">
                {(item.mean_change_pct * 100).toFixed(0)}% · {item.severity}
              </td>
              <td className="px-3 py-2 text-xs">
                {item.finding_id ? (
                  <button
                    type="button"
                    className="text-sky-400 underline"
                    onClick={() => {
                      const id = item.finding_id;
                      if (id) onOpenFinding(id);
                    }}
                  >
                    achado
                  </button>
                ) : null}
                {item.regression_alert_id ? (
                  <span className="ml-2 text-slate-400">
                    alerta {item.regression_alert_id.slice(0, 8)}
                  </span>
                ) : null}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}

export function itemsOf(
  digest: { items?: QueryDigestItem[] } | null,
  classification: QueryDigestItem["classification"],
): QueryDigestItem[] {
  return (digest?.items ?? []).filter(
    (item) => item.classification === classification,
  );
}
