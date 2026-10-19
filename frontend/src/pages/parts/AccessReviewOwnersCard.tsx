import { Button, Card, Input, Select } from "../../components/ui";
import type { AccessReviewOwner, AuditorAccount } from "../../types";

export function AccessReviewOwnersCard({
  owners,
  accounts,
  busy,
  ownerRole,
  ownerUserId,
  ownerContactName,
  ownerContactEmail,
  setOwnerRole,
  setOwnerUserId,
  setOwnerContactName,
  setOwnerContactEmail,
  onSave,
}: {
  owners: AccessReviewOwner[];
  accounts: AuditorAccount[];
  busy: boolean;
  ownerRole: string;
  ownerUserId: string;
  ownerContactName: string;
  ownerContactEmail: string;
  setOwnerRole: (v: string) => void;
  setOwnerUserId: (v: string) => void;
  setOwnerContactName: (v: string) => void;
  setOwnerContactEmail: (v: string) => void;
  onSave: () => void;
}) {
  return (
    <Card
      title="Responsáveis"
      subtitle="Conta do auditor (papel auditor+) por role/conta do banco. Contato opcional (PII)."
    >
      <ul className="mt-2 space-y-1 text-sm text-slate-300">
        {owners.map((o) => (
          <li key={o.id}>
            <span className="font-mono">{o.target_role}</span> →{" "}
            {o.owner_username}
            {o.contact_name || o.contact_email
              ? ` (${[o.contact_name, o.contact_email].filter(Boolean).join(" · ")})`
              : ""}
          </li>
        ))}
        {owners.length === 0 ? (
          <li>Nenhum mapeamento; linhas vão para o operator.</li>
        ) : null}
      </ul>
      <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <Input
          label="Role/conta alvo"
          value={ownerRole}
          onChange={(e) => setOwnerRole(e.target.value)}
        />
        <Select
          label="Responsável"
          value={ownerUserId}
          onChange={(e) => setOwnerUserId(e.target.value)}
          options={[
            { value: "", label: "Selecione" },
            ...accounts.map((a) => ({
              value: a.id,
              label: `${a.username} (${a.role})`,
            })),
          ]}
        />
        <Input
          label="Contato (nome)"
          value={ownerContactName}
          onChange={(e) => setOwnerContactName(e.target.value)}
        />
        <Input
          label="Contato (e-mail)"
          value={ownerContactEmail}
          onChange={(e) => setOwnerContactEmail(e.target.value)}
        />
        <div className="flex items-end">
          <Button
            disabled={busy || !ownerRole.trim() || !ownerUserId}
            onClick={onSave}
          >
            Adicionar
          </Button>
        </div>
      </div>
    </Card>
  );
}
