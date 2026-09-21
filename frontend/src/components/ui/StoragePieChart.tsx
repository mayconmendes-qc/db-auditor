/** Pie chart — shadcn Chart + Recharts (Custom Label pattern). */

import { useMemo } from "react";
import { Pie, PieChart } from "recharts";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "./chart";

export interface StoragePieSlice {
  label: string;
  size_bytes: number;
}

const PALETTE = [
  "#3b82f6",
  "#10b981",
  "#f59e0b",
  "#f9323f",
  "#a855f7",
  "#06b6d4",
  "#84cc16",
  "#ec4899",
];

function formatBytes(n: number): string {
  if (n <= 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function slugKey(label: string, idx: number): string {
  const base = label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  return base || `env-${idx}`;
}

export function StoragePieChart({ items }: { items: StoragePieSlice[] }) {
  const total = items.reduce((s, i) => s + Math.max(0, i.size_bytes), 0);

  const { chartData, chartConfig } = useMemo(() => {
    const config: ChartConfig = {
      size_bytes: { label: "Storage" },
    };
    const data = items.map((item, idx) => {
      const key = slugKey(item.label, idx);
      const color = PALETTE[idx % PALETTE.length];
      config[key] = { label: item.label, color };
      return {
        key,
        label: item.label,
        size_bytes: Math.max(0, item.size_bytes),
        fill: `var(--color-${key})`,
      };
    });
    return { chartData: data, chartConfig: config };
  }, [items]);

  if (items.length === 0 || total <= 0) {
    return <p className="text-sm text-slate-400">Sem dados de storage.</p>;
  }

  return (
    <div className="flex w-full flex-col gap-4 sm:flex-row sm:items-center">
      {/* Legenda: nome + armazenamento (sem %). */}
      <ul className="flex min-w-0 w-full flex-col justify-center space-y-3 sm:w-[42%]">
        {chartData.map((s) => (
          <li
            key={s.key}
            className="flex items-start gap-2.5 text-sm text-slate-300"
          >
            <span
              className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-sm"
              style={{
                backgroundColor: chartConfig[s.key]?.color ?? PALETTE[0],
              }}
              aria-hidden
            />
            <div className="min-w-0 flex-1">
              <span
                className="block truncate font-medium text-slate-100"
                title={s.label}
              >
                {s.label}
              </span>
              <p className="font-mono text-xs text-slate-400">
                {formatBytes(s.size_bytes)}
              </p>
            </div>
          </li>
        ))}
        <li className="border-t border-slate-800 pt-2 text-xs text-slate-500">
          Total:{" "}
          <span className="font-mono text-slate-300">{formatBytes(total)}</span>
        </li>
      </ul>

      {/* Gráfico: % apenas nos rótulos das fatias. */}
      <div className="flex w-full items-center justify-center sm:w-[58%]">
        <ChartContainer
          config={chartConfig}
          className="aspect-square h-[280px] w-full max-w-[300px]"
        >
          <PieChart margin={{ top: 12, right: 12, bottom: 12, left: 12 }}>
            <ChartTooltip
              content={
                <ChartTooltipContent
                  nameKey="key"
                  hideLabel
                  formatter={(value) => (
                    <span className="font-mono tabular-nums">
                      {formatBytes(Number(value))}
                    </span>
                  )}
                />
              }
            />
            <Pie
              data={chartData}
              dataKey="size_bytes"
              nameKey="key"
              cx="50%"
              cy="50%"
              outerRadius="78%"
              labelLine={false}
              label={({ payload, ...props }) => {
                const pct =
                  total > 0
                    ? ((Number(payload.size_bytes) / total) * 100).toFixed(0)
                    : "0";
                return (
                  <text
                    cx={props.cx}
                    cy={props.cy}
                    x={props.x}
                    y={props.y}
                    textAnchor={props.textAnchor}
                    dominantBaseline={props.dominantBaseline}
                    fill="#e8e8e8"
                    className="text-[12px] font-medium"
                  >
                    {pct}%
                  </text>
                );
              }}
            />
          </PieChart>
        </ChartContainer>
      </div>
    </div>
  );
}
