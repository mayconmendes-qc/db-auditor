import type { ReactNode } from "react";
import { Button } from "./Button";

export interface ModalProps {
  open: boolean;
  title: string;
  children: ReactNode;
  onClose: () => void;
  footer?: ReactNode;
}

export function Modal({ open, title, children, onClose, footer }: ModalProps) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
    >
      <div className="w-full max-w-lg rounded-xl border border-slate-700 bg-slate-900 shadow-xl">
        <header className="flex items-center justify-between border-b border-slate-700 px-5 py-4">
          <h2 id="modal-title" className="text-lg font-semibold text-slate-50">{title}</h2>
          <Button variant="ghost" onClick={onClose} aria-label="Fechar">✕</Button>
        </header>
        <div className="px-5 py-4 text-slate-300">{children}</div>
        {footer ? <footer className="flex justify-end gap-2 border-t border-slate-700 px-5 py-4">{footer}</footer> : null}
      </div>
    </div>
  );
}
