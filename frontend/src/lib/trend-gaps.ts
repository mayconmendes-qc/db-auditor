import type { RunTrendPoint } from "../types";

export type TrendEntry =
  | { kind: "point"; point: RunTrendPoint }
  | { kind: "gap"; key: string; environment: string; periods: number };

function bucketNumber(value: string, granularity: "day" | "week" | "month") {
  const date = new Date(value);
  if (granularity === "month")
    return date.getUTCFullYear() * 12 + date.getUTCMonth();
  const midnight = Date.UTC(
    date.getUTCFullYear(),
    date.getUTCMonth(),
    date.getUTCDate(),
  );
  if (granularity === "day") return Math.floor(midnight / 86400000);
  const monday = midnight - ((date.getUTCDay() + 6) % 7) * 86400000;
  return Math.floor(monday / (7 * 86400000));
}

export function trendEntries(
  points: RunTrendPoint[],
  granularity: "day" | "week" | "month",
): TrendEntry[] {
  const previous = new Map<string, number>();
  const entries: TrendEntry[] = [];
  for (const point of points) {
    const bucket = bucketNumber(point.at, granularity);
    const prior = previous.get(point.environment_id);
    if (prior != null && bucket - prior > 1) {
      entries.push({
        kind: "gap",
        key: `${point.environment_id}:${prior}:${bucket}`,
        environment: point.environment_name,
        periods: bucket - prior - 1,
      });
    }
    entries.push({ kind: "point", point });
    previous.set(point.environment_id, bucket);
  }
  return entries;
}
