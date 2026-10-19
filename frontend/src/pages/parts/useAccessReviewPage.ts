import { useCallback, useEffect, useState } from "react";
import type { PageSize } from "../../components/ui/PaginationControls";
import { useApp } from "../../context/AppContext";
import { formatError } from "../../lib/errors";
import { api } from "../../services/api";
import type {
  AccessReviewCampaign,
  AccessReviewDiff,
  AccessReviewItem,
  AccessReviewOwner,
  AccessReviewOwnerInput,
  AuditorAccount,
  AuditRun,
} from "../../types";

export function useAccessReviewPage() {
  const { environmentId, environments, setEnvironmentId } = useApp();
  const isOperator = api.hasRole("operator");
  const isAuditor = api.hasRole("auditor");

  const [campaigns, setCampaigns] = useState<AccessReviewCampaign[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [campaign, setCampaign] = useState<AccessReviewCampaign | null>(null);
  const [items, setItems] = useState<AccessReviewItem[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [pageSize, setPageSize] = useState<PageSize>(50);
  const [owners, setOwners] = useState<AccessReviewOwner[]>([]);
  const [accounts, setAccounts] = useState<AuditorAccount[]>([]);
  const [runs, setRuns] = useState<AuditRun[]>([]);
  const [diff, setDiff] = useState<AccessReviewDiff | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);

  const [filterAccount, setFilterAccount] = useState("");
  const [filterObject, setFilterObject] = useState("");
  const [filterPrivilege, setFilterPrivilege] = useState("");
  const [filterOrigin, setFilterOrigin] = useState("");
  const [filterHighlight, setFilterHighlight] = useState("");
  const [mineOnly, setMineOnly] = useState(false);
  const [pendingOnly, setPendingOnly] = useState(false);

  const [newName, setNewName] = useState("");
  const [newRunId, setNewRunId] = useState("");
  const [newDueAt, setNewDueAt] = useState("");
  const [ownerRole, setOwnerRole] = useState("");
  const [ownerUserId, setOwnerUserId] = useState("");
  const [ownerContactName, setOwnerContactName] = useState("");
  const [ownerContactEmail, setOwnerContactEmail] = useState("");

  const refreshCampaigns = useCallback(
    async (env: string, signal?: AbortSignal) => {
      const res = await api.accessReviews(env, { limit: 50 }, signal);
      setCampaigns(res.items);
      if (!selectedId && res.items[0]) setSelectedId(res.items[0].id);
    },
    [selectedId],
  );

  const refreshMatrix = useCallback(
    async (env: string, campaignId: string, signal?: AbortSignal) => {
      const limit = pageSize === "all" ? 100 : pageSize;
      const res = await api.accessReviewMatrix(
        env,
        campaignId,
        {
          account: filterAccount || undefined,
          object: filterObject || undefined,
          privilege: filterPrivilege || undefined,
          origin: filterOrigin || undefined,
          highlight: filterHighlight || undefined,
          pending: pendingOnly ? true : undefined,
          mine: mineOnly || undefined,
          limit,
          offset,
        },
        signal,
      );
      setItems(res.items);
      setTotal(res.total);
    },
    [
      filterAccount,
      filterHighlight,
      filterObject,
      filterOrigin,
      filterPrivilege,
      mineOnly,
      offset,
      pageSize,
      pendingOnly,
    ],
  );

  useEffect(() => {
    if (!environmentId) return;
    const ac = new AbortController();
    setLoading(true);
    setError(null);
    void (async () => {
      try {
        await refreshCampaigns(environmentId, ac.signal);
        const ownersRes = await api.accessReviewOwners(
          environmentId,
          ac.signal,
        );
        setOwners(ownersRes.items);
        if (isOperator) {
          const [users, runRes] = await Promise.all([
            api
              .accounts(ac.signal)
              .catch(() => ({ items: [] as AuditorAccount[] })),
            api.auditRuns(
              { environment_id: environmentId, status: "success" },
              ac.signal,
            ),
          ]);
          setAccounts(users.items.filter((u) => u.role !== "viewer"));
          setRuns(runRes.items);
          if (!newRunId && runRes.items[0]) setNewRunId(runRes.items[0].id);
        }
      } catch (err: unknown) {
        if (!ac.signal.aborted) {
          setError(formatError(err, "Falha ao carregar revisão de acessos."));
        }
      } finally {
        if (!ac.signal.aborted) setLoading(false);
      }
    })();
    return () => ac.abort();
  }, [environmentId, isOperator, refreshCampaigns, newRunId]);

  useEffect(() => {
    if (!environmentId || !selectedId) {
      setCampaign(null);
      setItems([]);
      setDiff(null);
      return;
    }
    const ac = new AbortController();
    void (async () => {
      try {
        const [c] = await Promise.all([
          api.accessReview(environmentId, selectedId, ac.signal),
          refreshMatrix(environmentId, selectedId, ac.signal),
        ]);
        setCampaign(c);
        const d = await api.accessReviewDiff(
          environmentId,
          selectedId,
          ac.signal,
        );
        setDiff(d);
      } catch (err: unknown) {
        if (!ac.signal.aborted) {
          setError(formatError(err, "Falha ao carregar campanha."));
        }
      }
    })();
    return () => ac.abort();
  }, [environmentId, selectedId, refreshMatrix]);

  const createCampaign = async () => {
    if (!environmentId || !newName.trim() || !newRunId || !newDueAt) return;
    setBusy(true);
    setError(null);
    try {
      const created = await api.createAccessReview(environmentId, {
        name: newName.trim(),
        reference_run_id: newRunId,
        due_at: new Date(newDueAt).toISOString(),
      });
      setNewName("");
      setSelectedId(created.id);
      await refreshCampaigns(environmentId);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao criar campanha."));
    } finally {
      setBusy(false);
    }
  };

  const saveOwner = async () => {
    if (!environmentId || !ownerRole.trim() || !ownerUserId) return;
    setBusy(true);
    setError(null);
    try {
      const next: AccessReviewOwnerInput[] = [
        ...owners.map((o) => ({
          target_role: o.target_role,
          owner_user_id: o.owner_user_id,
          contact_name: o.contact_name,
          contact_email: o.contact_email,
        })),
        {
          target_role: ownerRole.trim(),
          owner_user_id: ownerUserId,
          contact_name: ownerContactName.trim() || undefined,
          contact_email: ownerContactEmail.trim() || undefined,
        },
      ];
      const res = await api.replaceAccessReviewOwners(environmentId, next);
      setOwners(res.items);
      setOwnerRole("");
      setOwnerContactName("");
      setOwnerContactEmail("");
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao gravar responsáveis."));
    } finally {
      setBusy(false);
    }
  };

  const closeCampaign = async () => {
    if (!environmentId || !selectedId) return;
    setBusy(true);
    try {
      const closed = await api.closeAccessReview(environmentId, selectedId);
      setCampaign(closed);
      await refreshCampaigns(environmentId);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao encerrar campanha."));
    } finally {
      setBusy(false);
    }
  };

  const exportCSV = async () => {
    if (!environmentId || !selectedId) return;
    setBusy(true);
    try {
      const text = await api.accessReviewExportCSV(environmentId, selectedId);
      const blob = new Blob([text], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `access-review-${selectedId}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao exportar CSV."));
    } finally {
      setBusy(false);
    }
  };

  const requestPDF = async () => {
    if (!environmentId || !campaign) return;
    setBusy(true);
    try {
      await api.requestPDFReport(
        environmentId,
        campaign.reference_run_id,
        "access_review",
        { campaign_id: campaign.id, redaction: "identifiers" },
      );
    } catch (err: unknown) {
      setError(formatError(err, "Falha ao solicitar PDF."));
    } finally {
      setBusy(false);
    }
  };

  const onDecided = () => {
    if (environmentId && selectedId) {
      void refreshMatrix(environmentId, selectedId);
      void api.accessReview(environmentId, selectedId).then(setCampaign);
    }
  };

  return {
    environmentId,
    environments,
    setEnvironmentId,
    isOperator,
    isAuditor,
    campaigns,
    selectedId,
    setSelectedId,
    campaign,
    items,
    total,
    offset,
    setOffset,
    pageSize,
    setPageSize,
    owners,
    accounts,
    runs,
    diff,
    error,
    setError,
    loading,
    busy,
    filterAccount,
    setFilterAccount,
    filterObject,
    setFilterObject,
    filterPrivilege,
    setFilterPrivilege,
    filterOrigin,
    setFilterOrigin,
    filterHighlight,
    setFilterHighlight,
    mineOnly,
    setMineOnly,
    pendingOnly,
    setPendingOnly,
    newName,
    setNewName,
    newRunId,
    setNewRunId,
    newDueAt,
    setNewDueAt,
    ownerRole,
    setOwnerRole,
    ownerUserId,
    setOwnerUserId,
    ownerContactName,
    setOwnerContactName,
    ownerContactEmail,
    setOwnerContactEmail,
    createCampaign,
    saveOwner,
    closeCampaign,
    exportCSV,
    requestPDF,
    onDecided,
  };
}
