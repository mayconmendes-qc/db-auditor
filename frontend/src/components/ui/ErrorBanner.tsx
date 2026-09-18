import type { ReactNode } from "react";
import { Button } from "./Button";

export interface ErrorBannerProps {
  title?: string;
  message: string;
  onRetry?: () => void;
  children?: ReactNode;
}

/** Friendly error surface for page-level failures. */
export function ErrorBanner({
  title = "Não foi possível carregar",
  message,
  onRetry,
  children,
}: ErrorBannerProps) {
  return (
    <div
      className="rounded-xl border border-rose-800/60 bg-rose-950/40 px-4 py-4 text-sm text-rose-100"
      role="alert"
    >
      <p className="font-semibold text-rose-50">{title}</p>
      <p className="mt-1 leading-relaxed text-rose-200/90">{message}</p>
      {children}
      {onRetry ? (
        <div className="mt-3">
          <Button type="button" onClick={onRetry}>
            Tentar novamente
          </Button>
        </div>
      ) : null}
    </div>
  );
}
