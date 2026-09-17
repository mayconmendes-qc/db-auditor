export const navigationSections = [
  "Visão geral",
  "Ambientes",
  "Audit runs",
  "Inventário",
  "Findings",
];

export function App() {
  return (
    <main className="shell">
      <aside>
        <strong>Timescale Auditor</strong>
        <nav>
          {navigationSections.map((section) => (
            <a href={`#${section}`} key={section}>
              {section}
            </a>
          ))}
        </nav>
      </aside>
      <section className="content">
        <p className="eyebrow">SPRINT 0</p>
        <h1>Auditoria com evidências, não mudanças automáticas.</h1>
        <p>
          O painel consumirá exclusivamente a API do Timescale Auditor. A coleta
          dos ambientes permanece somente leitura.
        </p>
        <div className="cards">
          <article>
            <span>API</span>
            <strong>Em preparação</strong>
          </article>
          <article>
            <span>Snapshot Store</span>
            <strong>Em preparação</strong>
          </article>
          <article>
            <span>Próximo passo</span>
            <strong>Registrar ambientes</strong>
          </article>
        </div>
      </section>
    </main>
  );
}
