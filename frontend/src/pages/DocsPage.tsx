import { PageHeader } from "../components/PageHeader";
import { Accordion, Button } from "../components/ui";
import type { NavigationSection } from "../types";
import { type DocTopic, docTopics } from "./parts/docsTopics";

export interface DocsPageProps {
  onNavigate?: (section: NavigationSection) => void;
}

function OpenTopic({
  topic,
  onNavigate,
}: {
  topic: NavigationSection;
  onNavigate: (section: NavigationSection) => void;
}) {
  return (
    <div className="pt-1">
      <Button type="button" onClick={() => onNavigate(topic)}>
        Abrir {topic}
      </Button>
    </div>
  );
}

function TopicBody({
  topic,
  onNavigate,
}: {
  topic: DocTopic;
  onNavigate?: (section: NavigationSection) => void;
}) {
  return (
    <div className="space-y-8">
      <p>{topic.intro}</p>
      <div>
        <h4 className="text-xs font-medium tracking-wide text-slate-400">
          Como usar
        </h4>
        <ol className="mt-3 list-decimal space-y-3 pl-5">
          {topic.steps.map((step) => (
            <li key={step} className="pl-1">
              {step}
            </li>
          ))}
        </ol>
      </div>
      {topic.notes.length > 0 ? (
        <div>
          <h4 className="text-xs font-medium tracking-wide text-slate-400">
            Vale saber
          </h4>
          <ul className="mt-3 list-disc space-y-3 pl-5">
            {topic.notes.map((note) => (
              <li key={note} className="pl-1">
                {note}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {topic.terms.length > 0 ? (
        <div>
          <h4 className="text-xs font-medium tracking-wide text-slate-400">
            Termos desta tela
          </h4>
          <ul className="mt-3 space-y-3">
            {topic.terms.map((term) => (
              <li key={term.name}>
                <span className="font-medium text-slate-100">{term.name}</span>
                {" — "}
                {term.meaning}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {topic.goTo && onNavigate ? (
        <OpenTopic topic={topic.goTo} onNavigate={onNavigate} />
      ) : null}
    </div>
  );
}

export function DocsPage({ onNavigate }: DocsPageProps) {
  return (
    <div className="space-y-4">
      <PageHeader
        eyebrow="Ajuda"
        title="Documentação"
        description="Escolha um tópico. O texto abre logo abaixo do título, com o passo a passo e o significado dos termos. Clique de novo para fechar. O DB Auditor só observa: ele não altera o banco analisado."
      />
      <Accordion
        items={docTopics.map((topic) => ({
          id: topic.id,
          title: topic.title,
          content: <TopicBody topic={topic} onNavigate={onNavigate} />,
        }))}
        defaultOpenId="comecar"
      />
    </div>
  );
}
