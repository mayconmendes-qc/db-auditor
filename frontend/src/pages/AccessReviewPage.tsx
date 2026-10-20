import { PageHeader } from "../components/PageHeader";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Input,
  Select,
  Skeleton,
} from "../components/ui";
import { AccessReviewMatrix } from "./parts/AccessReviewMatrix";
import { AccessReviewOwnersCard } from "./parts/AccessReviewOwnersCard";
import { useAccessReviewPage } from "./parts/useAccessReviewPage";

export function AccessReviewPage() {
  const s = useAccessReviewPage();

  return (
    <>
      <PageHeader
        eyebrow="Análise"
        title="Revisão de acessos"
        description="Campanhas com matriz congelada de privilégios efetivos (origem e coluna), decisões imutáveis e exportação de evidência."
      />
      <div className="mt-8 space-y-5">
        <Select
          label="Ambiente"
          value={s.environmentId ?? ""}
          onChange={(e) => s.setEnvironmentId(e.target.value || null)}
          options={[
            { value: "", label: "Selecione um ambiente" },
            ...s.environments.map((env) => ({
              value: env.id,
              label: env.name,
            })),
          ]}
        />
        {s.error ? (
          <ErrorBanner message={s.error} onRetry={() => s.setError(null)} />
        ) : null}
        {s.loading ? <Skeleton className="h-36" /> : null}
        {!s.environmentId ? (
          <EmptyState
            title="Selecione um ambiente"
            description="Escolha o ambiente para ver campanhas de revisão."
          />
        ) : null}

        {s.environmentId && !s.loading ? (
          <>
            {s.isOperator ? (
              <Card
                title="Nova campanha"
                subtitle="Exige coleta de referência completa. Auditor não cria campanha."
              >
                <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  <Input
                    label="Nome"
                    value={s.newName}
                    onChange={(e) => s.setNewName(e.target.value)}
                  />
                  <Select
                    label="Coleta de referência"
                    value={s.newRunId}
                    onChange={(e) => s.setNewRunId(e.target.value)}
                    options={[
                      { value: "", label: "Selecione" },
                      ...s.runs.map((r) => ({
                        value: r.id,
                        label: `${r.id.slice(0, 8)}… · ${r.status}`,
                      })),
                    ]}
                  />
                  <Input
                    label="Prazo"
                    type="datetime-local"
                    value={s.newDueAt}
                    onChange={(e) => s.setNewDueAt(e.target.value)}
                  />
                  <div className="flex items-end">
                    <Button
                      disabled={
                        s.busy ||
                        !s.newName.trim() ||
                        !s.newRunId ||
                        !s.newDueAt
                      }
                      onClick={() => void s.createCampaign()}
                    >
                      Criar campanha
                    </Button>
                  </div>
                </div>
              </Card>
            ) : null}

            <Card title="Campanhas" subtitle="Progresso e estado por ambiente.">
              {s.campaigns.length === 0 ? (
                <EmptyState
                  className="mt-3"
                  title="Nenhuma campanha"
                  description={
                    s.isOperator
                      ? "Crie a primeira campanha com uma coleta completa."
                      : "Aguarde um operator criar a campanha."
                  }
                />
              ) : (
                <Select
                  label="Campanha"
                  value={s.selectedId}
                  onChange={(e) => {
                    s.setSelectedId(e.target.value);
                    s.setOffset(0);
                  }}
                  options={s.campaigns.map((c) => ({
                    value: c.id,
                    label: `${c.name} · ${c.status} · ${c.decided_percent.toFixed(0)}%`,
                  }))}
                />
              )}
              {s.campaign ? (
                <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-200">
                  <Badge
                    tone={
                      s.campaign.status === "closed"
                        ? "success"
                        : s.campaign.status === "overdue"
                          ? "danger"
                          : "neutral"
                    }
                  >
                    {s.campaign.status}
                  </Badge>
                  <span>
                    {s.campaign.decided_items}/{s.campaign.total_items}{" "}
                    decididos ({s.campaign.decided_percent.toFixed(1)}%) ·
                    pendências {s.campaign.pending_items}
                  </span>
                  <span>
                    Prazo: {new Date(s.campaign.due_at).toLocaleString()}
                  </span>
                  <div className="flex flex-wrap gap-2">
                    {s.isOperator && s.campaign.status !== "closed" ? (
                      <Button
                        disabled={s.busy}
                        onClick={() => void s.closeCampaign()}
                      >
                        Encerrar
                      </Button>
                    ) : null}
                    {s.isAuditor ? (
                      <>
                        <Button
                          disabled={s.busy}
                          onClick={() => void s.exportCSV()}
                        >
                          Exportar CSV
                        </Button>
                        <Button
                          disabled={s.busy}
                          onClick={() => void s.requestPDF()}
                        >
                          PDF de evidência
                        </Button>
                      </>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </Card>

            {s.isOperator ? (
              <AccessReviewOwnersCard
                owners={s.owners}
                accounts={s.accounts}
                busy={s.busy}
                ownerRole={s.ownerRole}
                ownerUserId={s.ownerUserId}
                ownerContactName={s.ownerContactName}
                ownerContactEmail={s.ownerContactEmail}
                setOwnerRole={s.setOwnerRole}
                setOwnerUserId={s.setOwnerUserId}
                setOwnerContactName={s.setOwnerContactName}
                setOwnerContactEmail={s.setOwnerContactEmail}
                onSave={() => void s.saveOwner()}
              />
            ) : null}

            {s.selectedId && s.environmentId ? (
              <Card
                title="Matriz"
                subtitle="Filtros no servidor. Origem: direto / via role / PUBLIC / predefinida."
              >
                <div className="mb-3 flex flex-wrap gap-3">
                  <Input
                    label="Conta"
                    value={s.filterAccount}
                    onChange={(e) => {
                      s.setFilterAccount(e.target.value);
                      s.setOffset(0);
                    }}
                  />
                  <Input
                    label="Objeto"
                    value={s.filterObject}
                    onChange={(e) => {
                      s.setFilterObject(e.target.value);
                      s.setOffset(0);
                    }}
                  />
                  <Input
                    label="Privilégio"
                    value={s.filterPrivilege}
                    onChange={(e) => {
                      s.setFilterPrivilege(e.target.value);
                      s.setOffset(0);
                    }}
                  />
                  <Select
                    label="Origem"
                    value={s.filterOrigin}
                    onChange={(e) => {
                      s.setFilterOrigin(e.target.value);
                      s.setOffset(0);
                    }}
                    options={[
                      { value: "", label: "Todas" },
                      { value: "direct", label: "direto" },
                      { value: "role", label: "via role" },
                      { value: "public", label: "PUBLIC" },
                      { value: "predefined", label: "predefinida" },
                    ]}
                  />
                  <Select
                    label="Destaque"
                    value={s.filterHighlight}
                    onChange={(e) => {
                      s.setFilterHighlight(e.target.value);
                      s.setOffset(0);
                    }}
                    options={[
                      { value: "", label: "Todos" },
                      { value: "powerful_role", label: "powerful_role" },
                      {
                        value: "inactive_or_expired",
                        label: "inactive_or_expired",
                      },
                      {
                        value: "new_since_previous",
                        label: "new_since_previous",
                      },
                      { value: "cross_app_write", label: "cross_app_write" },
                    ]}
                  />
                  <label className="flex items-end gap-2 pb-2 text-sm text-slate-300">
                    <input
                      type="checkbox"
                      checked={s.pendingOnly}
                      onChange={(e) => {
                        s.setPendingOnly(e.target.checked);
                        s.setOffset(0);
                      }}
                    />
                    Só pendentes
                  </label>
                  {s.isAuditor ? (
                    <label className="flex items-end gap-2 pb-2 text-sm text-slate-300">
                      <input
                        type="checkbox"
                        checked={s.mineOnly}
                        onChange={(e) => {
                          s.setMineOnly(e.target.checked);
                          s.setOffset(0);
                        }}
                      />
                      Minhas pendências
                    </label>
                  ) : null}
                </div>
                <AccessReviewMatrix
                  items={s.items}
                  total={s.total}
                  offset={s.offset}
                  pageSize={s.pageSize}
                  busy={s.busy}
                  campaignClosed={s.campaign?.status === "closed"}
                  environmentId={s.environmentId}
                  campaignId={s.selectedId}
                  onPageSizeChange={(size) => {
                    s.setPageSize(size);
                    s.setOffset(0);
                  }}
                  onOffsetChange={s.setOffset}
                  onDecided={s.onDecided}
                  onError={(msg) => s.setError(msg)}
                />
              </Card>
            ) : null}

            {s.diff ? (
              <Card
                title="Diff com a campanha anterior"
                subtitle={
                  s.diff.previous_campaign_id
                    ? `Anterior: ${s.diff.previous_campaign_id.slice(0, 8)}…`
                    : "Sem campanha anterior"
                }
              >
                <div className="mt-2 grid gap-3 text-sm text-slate-300 sm:grid-cols-2">
                  <p>Privilégios novos: {s.diff.new_privileges.length}</p>
                  <p>
                    Privilégios removidos: {s.diff.removed_privileges.length}
                  </p>
                  <p>Decisões alteradas: {s.diff.changed_decisions.length}</p>
                  <p>
                    Contas novas/removidas: {s.diff.new_accounts.length}/
                    {s.diff.removed_accounts.length}
                  </p>
                </div>
                {s.diff.new_privileges.slice(0, 10).map((p) => (
                  <p
                    key={p.id}
                    className="mt-1 font-mono text-xs text-amber-200"
                  >
                    + {p.account_name} {p.privilege} em {p.object_name} (
                    {p.origin_label})
                  </p>
                ))}
              </Card>
            ) : null}
          </>
        ) : null}
      </div>
    </>
  );
}
