import type { ComponentChildren } from "preact";
import type { Project } from "../../domain/models";
import { PageHeader } from "../components/PageHeader";
import { ProjectNav } from "../features/project/ProjectNav";
import { SampleStrip } from "../features/samples/Samples";
import { useAsync } from "../hooks/useAsync";
import { useBuild, useUseCases } from "../services/context";
import { useI18n } from "../i18n/I18nProvider";
import type { ProjectPage } from "../router/routes";
import styles from "./Page.module.css";

interface Props {
  readonly project: Project;
  readonly current: "overview" | ProjectPage;
  readonly runId?: string | undefined;
  readonly title?: string;
  readonly subtitle?: ComponentChildren;
  readonly actions?: ComponentChildren;
  readonly children: ComponentChildren;
}

/** Header and section navigation shared by every project screen. */
export function ProjectFrame({ project, current, runId, title, subtitle, actions, children }: Props) {
  const { t } = useI18n();
  const build = useBuild();
  const { samples } = useUseCases();
  const scenario = useAsync(() => samples.forProject(build, project.id), [samples, build, project.id]);
  return (
    <>
      <PageHeader
        back={{ href: "#/", label: t("nav.backToProjects") }}
        eyebrow={title ? project.name : t("project.title")}
        title={title ?? project.name}
        subtitle={subtitle}
        actions={actions}
      />
      {scenario.status === "ok" && scenario.data && current !== "input" && <SampleStrip scenario={scenario.data} />}
      <ProjectNav projectId={project.id} current={current} runId={runId} />
      <div class={styles.stack}>{children}</div>
    </>
  );
}
