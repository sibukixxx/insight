import type { InputWorkspace } from "../../application/usecases/importEvidence";
import { ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Loading, PageError } from "../components/States";
import { CsvImport } from "../features/input/CsvImport";
import { DocumentList } from "../features/input/DocumentList";
import { FormatGuide } from "../features/input/FormatGuide";
import { TextEvidenceForm } from "../features/input/TextEvidenceForm";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./Page.module.css";

export function InputPage({ projectId }: { projectId: string }) {
  const { input } = useUseCases();
  const state = useAsync(() => input.loadInput(projectId), [input, projectId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  return <InputView data={state.data} onChanged={state.reload} />;
}

function InputView({ data, onChanged }: { data: InputWorkspace; onChanged: () => void }) {
  const { t } = useI18n();
  const { project, documents, formats } = data;
  const documentsFormat = formats.find((f) => f.kind === "documents");
  return (
    <ProjectFrame project={project} current="input" title={t("input.title")} subtitle={t("input.lead")}>
      {documents.length > 0 && (
        <div class={styles.nextAction} role="region" aria-label={t("nextStep.label")}>
          <span class={styles.nextTitle}>{t("input.ready", { count: documents.length })}</span>
          <ButtonLink variant="primary" href={projectHash(project.id, "analysis")}>{t("nextStep.analysis.action")} {"→"}</ButtonLink>
        </div>
      )}
      <Card title={t("input.formats.title")} description={t("input.formats.hint")}>
        <FormatGuide formats={formats} />
      </Card>
      <div class={styles.split}>
        <Card title={t("project.importCsv")} description={t("input.csv.hint")}>
          <CsvImport projectId={project.id} formats={formats} onImported={onChanged} />
        </Card>
        <Card title={t("project.pasteText")} description={t("input.text.hint")}>
          <TextEvidenceForm projectId={project.id} sourceTypes={documentsFormat?.sourceTypes ?? []} onAdded={onChanged} />
        </Card>
      </div>
      <Card title={t("project.documents")} description={t("input.documentsCount", { count: documents.length })}>
        <DocumentList documents={documents} />
      </Card>
    </ProjectFrame>
  );
}
