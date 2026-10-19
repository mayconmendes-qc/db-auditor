import { useEffect, useState } from "react";
import {
  Button,
  EmptyState,
  Input,
  Skeleton,
  Table,
} from "../../components/ui";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type { EnvAPIToken } from "../../types";

const TOKEN_SCOPES = [
  "snapshot-context:read",
  "migration-analysis:write",
  "deploy-annotation:write",
  "quality-contract:run",
  "restore-evidence:write",
  "last-login:write",
] as const;

export function EnvAPITokenCard({ environmentId }: { environmentId: string }) {
  const [items, setItems] = useState<EnvAPIToken[] | null>(null);
  const [name, setName] = useState("ci");
  const [scopes, setScopes] = useState<string[]>([
    "snapshot-context:read",
    "migration-analysis:write",
  ]);
  const [plaintext, setPlaintext] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setItems(null);
    api
      .listEnvAPITokens(environmentId)
      .then((res) => {
        if (!cancelled) setItems(res.items);
      })
      .catch((err: unknown) =>
        setError(formatError(err, "Não foi possível listar tokens")),
      );
    return () => {
      cancelled = true;
    };
  }, [environmentId, reload]);

  return (
    <div className="mt-8 space-y-3 rounded-lg border border-slate-800 p-4">
      <h3 className="text-lg font-semibold text-slate-100">Tokens de API</h3>
      <p className="text-sm text-slate-400">
        Credencial por ambiente para a CLI (ADR-0001 D1). O segredo aparece uma
        vez. Guarde em GitHub Secrets — nunca em log.
      </p>
      {error ? <p className="text-sm text-rose-300">{error}</p> : null}
      {plaintext ? (
        <div className="rounded-md border border-amber-700/60 bg-amber-950/40 p-3 text-sm text-amber-100">
          <p className="font-medium">Copie agora — não será exibido de novo:</p>
          <code className="mt-2 block break-all">{plaintext}</code>
          <Button
            type="button"
            className="mt-2"
            onClick={() => setPlaintext(null)}
          >
            Ocultar
          </Button>
        </div>
      ) : null}
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          setError(null);
          void api
            .createEnvAPIToken(environmentId, { name, scopes })
            .then((res) => {
              setPlaintext(res.plaintext);
              setReload((n) => n + 1);
            })
            .catch((err: unknown) =>
              setError(formatError(err, "Falha ao criar token")),
            );
        }}
      >
        <Input
          label="Nome"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <div className="flex flex-wrap gap-2 text-xs text-slate-300">
          {TOKEN_SCOPES.map((s) => (
            <label key={s} className="inline-flex items-center gap-1">
              <input
                type="checkbox"
                checked={scopes.includes(s)}
                onChange={(e) => {
                  setScopes((prev) =>
                    e.target.checked
                      ? [...prev, s]
                      : prev.filter((x) => x !== s),
                  );
                }}
              />
              {s}
            </label>
          ))}
        </div>
        <Button type="submit">Criar token</Button>
      </form>
      {items === null ? (
        <Skeleton className="h-24 w-full" />
      ) : items.length === 0 ? (
        <EmptyState
          title="Nenhum token"
          description="Crie um token com escopos mínimos para o CI."
        />
      ) : (
        <Table
          headers={[
            { id: "name", label: "Nome" },
            { id: "prefix", label: "Prefixo" },
            { id: "scopes", label: "Escopos" },
            { id: "exp", label: "Expira" },
            { id: "actions", label: "" },
          ]}
        >
          {items.map((t) => (
            <tr key={t.id} className="border-t border-slate-800">
              <td className="px-4 py-3 text-slate-100">{t.name}</td>
              <td className="px-4 py-3 text-slate-300">{t.public_prefix}</td>
              <td className="px-4 py-3 text-slate-300">
                {t.scopes.join(", ")}
              </td>
              <td className="px-4 py-3 text-slate-300">
                {t.revoked_at
                  ? "revogado"
                  : new Date(t.expires_at).toLocaleDateString()}
              </td>
              <td className="px-4 py-3">
                {!t.revoked_at ? (
                  <Button
                    type="button"
                    onClick={() => {
                      void api
                        .revokeEnvAPIToken(environmentId, t.id)
                        .then(() => setReload((n) => n + 1))
                        .catch((err: unknown) =>
                          setError(formatError(err, "Falha ao revogar")),
                        );
                    }}
                  >
                    Revogar
                  </Button>
                ) : null}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  );
}
