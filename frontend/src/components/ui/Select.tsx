import type { SelectHTMLAttributes } from "react";

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  options: Array<{ value: string; label: string }>;
}

export function Select({
  label,
  options,
  className = "",
  id,
  ...props
}: SelectProps) {
  const selectId = id ?? props.name;
  return (
    <label className="grid gap-1.5 text-sm text-slate-300">
      {label ? <span>{label}</span> : null}
      <select
        id={selectId}
        className={`rounded-md border border-slate-600 bg-slate-950 px-3 py-2 text-slate-100 focus:border-emerald-400 focus:outline-none focus:ring-1 focus:ring-emerald-400 ${className}`}
        {...props}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}
