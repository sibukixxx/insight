import type { InputWorkspace } from "../../application/usecases/importEvidence";
import { ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Loading, PageError } from "../components/States";
import { CsvImport, type CsvSample } from "../features/input/CsvImport";
import { SampleScenarioPanel } from "../features/samples/Samples";
import { DocumentList } from "../features/input/DocumentList";
import { TextEvidenceForm } from "../features/input/TextEvidenceForm";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { useBuild, useUseCases } from "../services/context";
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
  const build = useBuild();
  const { samples, exports } = useUseCases();
  const { project, documents, formats } = data;
  const scenarioState = useAsync(() => samples.forProject(build, project.id), [samples, build, project.id]);
  const scenario = scenarioState.status === "ok" ? scenarioState.data : undefined;
  const sample: CsvSample | undefined = scenario && {
    kind: scenario.importKind, rows: scenario.rows, downloadHref: exports.sampleInputLink(scenario.id),
    load: () => samples.loadInput(scenario.id),
  };
  const documentsFormat = formats.find((f) => f.kind === "documents");
  return (
    <ProjectFrame project={project} current="input" title={t("input.title")} subtitle={t("input.lead")}>
      {documents.length > 0 && (
        <div class={styles.nextAction} role="region" aria-label={t("nextStep.label")}>
          <span class={styles.nextTitle}>{t("input.ready", { count: documents.length })}</span>
          <ButtonLink variant="primary" href={projectHash(project.id, "analysis")}>{t("nextStep.analysis.action")} {"→"}</ButtonLink>
        </div>
      )}
      {scenario && <SampleScenarioPanel scenario={scenario} hasDocuments={documents.length > 0} />}
      <p class={styles.hint}>{t("input.formats.title")}{": "}{t("input.formats.hint")}</p>
      <div class={styles.stack}>
        <Card title={t("project.importCsv")} description={t("input.csv.hint")}>
          <CsvImport projectId={project.id} formats={formats} onImported={onChanged} sample={sample} />
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
