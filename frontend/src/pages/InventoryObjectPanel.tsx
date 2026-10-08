import { type ReactNode, useState } from "react";
import {
  type DetailAvailability,
  InventoryDetailAvailability,
  type InventoryDetailTab,
  InventoryDetailTabs,
} from "../components/InventoryDetailTabs";
import {
  type InventoryTarget,
  inventoryPermalink,
} from "../lib/inventoryTarget";
import type { SnapshotCompleteness } from "../types";

/** Shared shell for object types whose specialized assessments are delivered in later stages. */
export function InventoryObjectPanel({
  target,
  environment,
  run,
  snapshot,
  overview,
  sections = {},
  sectionStates = {},
}: {
  target: InventoryTarget;
  environment: string;
  run: string | null;
  snapshot: SnapshotCompleteness | null;
  overview: ReactNode;
  sections?: Partial<Record<InventoryDetailTab, ReactNode>>;
  sectionStates?: Partial<Record<InventoryDetailTab, DetailAvailability>>;
}) {
  const [tab, setTab] = useState<InventoryDetailTab>("overview");
  const coverageState =
    snapshot?.completeness === "partial"
      ? "partial"
      : snapshot?.completeness === "empty"
        ? "empty"
        : null;
  return (
    <div className="space-y-4">
      {run ? (
        <a
          href={inventoryPermalink(environment, run, target)}
          className="text-xs text-cyan-300 underline"
        >
          Link direto para este objeto e execução
        </a>
      ) : (
        <InventoryDetailAvailability
          state="not_collected"
          detail="A execução de origem não está disponível; não é possível criar um link estável."
        />
      )}
      {coverageState ? (
        <InventoryDetailAvailability state={coverageState} />
      ) : null}
      <InventoryDetailTabs tab={tab} onTabChange={setTab}>
        {tab === "overview"
          ? overview
          : (sections[tab] ?? (
              <InventoryDetailAvailability
                state={sectionStates[tab] ?? "not_collected"}
              />
            ))}
      </InventoryDetailTabs>
    </div>
  );
}
