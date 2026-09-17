import { navigationSections, Shell } from "./components/layout/Shell";
import { Card } from "./components/ui";

export { navigationSections };

export function App() {
  return (
    <Shell activeSection="Visão geral">
      <p className="text-xs font-bold tracking-[0.12em] text-emerald-300">
        SPRINT 0
      </p>
      <h1 className="mt-2 max-w-2xl text-3xl font-semibold leading-tight text-slate-50 md:text-5xl">
        Auditoria com evidências, não mudanças automáticas.
      </h1>
      <p className="mt-4 max-w-2xl text-base leading-relaxed text-slate-400">
        O painel consome exclusivamente a API do Timescale Auditor. A coleta dos
        ambientes permanece somente leitura.
      </p>
      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card subtitle="API" title="Health / Ready" />
        <Card subtitle="Snapshot Store" title="PostgreSQL interno" />
        <Card subtitle="Próximo passo" title="Registrar ambientes" />
      </div>
    </Shell>
  );
}
