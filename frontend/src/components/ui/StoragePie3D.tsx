/** SVG pie with slight 3D extrusion — no chart library dependency. */

export interface StoragePieSlice {
  label: string;
  size_bytes: number;
}

const COLORS = [
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

function polar(cx: number, cy: number, r: number, angle: number) {
  const rad = ((angle - 90) * Math.PI) / 180;
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
}

function arcPath(
  cx: number,
  cy: number,
  r: number,
  startAngle: number,
  endAngle: number,
): string {
  const start = polar(cx, cy, r, endAngle);
  const end = polar(cx, cy, r, startAngle);
  const large = endAngle - startAngle > 180 ? 1 : 0;
  return `M ${cx} ${cy} L ${end.x} ${end.y} A ${r} ${r} 0 ${large} 1 ${start.x} ${start.y} Z`;
}

export function StoragePie3D({ items }: { items: StoragePieSlice[] }) {
  const total = items.reduce((s, i) => s + Math.max(0, i.size_bytes), 0);
  if (items.length === 0 || total <= 0) {
    return (
      <p className="text-sm text-slate-400">Sem dados de storage.</p>
    );
  }

  const cx = 100;
  const cy = 88;
  const r = 70;
  const depth = 12;

  let angle = 0;
  const slices = items.map((item, idx) => {
    const value = Math.max(0, item.size_bytes);
    const pct = (value / total) * 100;
    const sweep = (value / total) * 360;
    const start = angle;
    const end = angle + sweep;
    angle = end;
    return {
      ...item,
      pct,
      start,
      end,
      color: COLORS[idx % COLORS.length],
    };
  });

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
      <div
        className="relative mx-auto shrink-0"
        style={{ width: 200, height: 200, perspective: "420px" }}
      >
        <svg
          viewBox="0 0 200 200"
          width={200}
          height={200}
          className="overflow-visible"
          style={{
            transform: "rotateX(52deg)",
            transformOrigin: "center center",
          }}
          role="img"
          aria-label="Storage por ambiente"
        >
          {/* underside / extrusion */}
          <g opacity={0.55}>
            {slices.map((s) => {
              if (s.end - s.start < 0.2) {
                return null;
              }
              return (
                <path
                  key={`d-${s.label}`}
                  d={arcPath(cx, cy + depth, r, s.start, s.end)}
                  fill={s.color}
                />
              );
            })}
          </g>
          {/* top face */}
          {slices.map((s) => {
            if (s.end - s.start < 0.2) {
              return null;
            }
            return (
              <path
                key={s.label}
                d={arcPath(cx, cy, r, s.start, s.end)}
                fill={s.color}
                stroke="#121212"
                strokeWidth={1}
              >
                <title>
                  {s.label}: {formatBytes(s.size_bytes)} ({s.pct.toFixed(1)}%)
                </title>
              </path>
            );
          })}
        </svg>
      </div>

      <ul className="min-w-0 flex-1 space-y-2">
        {slices.map((s) => (
          <li
            key={s.label}
            className="flex items-start gap-2 text-sm text-slate-300"
          >
            <span
              className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-sm"
              style={{ backgroundColor: s.color }}
              aria-hidden
            />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5">
                <span className="truncate font-medium text-slate-100" title={s.label}>
                  {s.label}
                </span>
                <span className="shrink-0 font-mono text-xs text-slate-200">
                  {s.pct.toFixed(1)}%
                </span>
              </div>
              <p className="font-mono text-xs text-slate-400">
                {formatBytes(s.size_bytes)}
              </p>
            </div>
          </li>
        ))}
        <li className="border-t border-slate-800 pt-2 text-xs text-slate-500">
          Total: {" "}
          <span className="font-mono text-slate-300">{formatBytes(total)}</span>
        </li>
      </ul>
    </div>
  );
}
