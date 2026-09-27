import type { ComponentChildren } from "preact";
import type { Project } from "../../domain/models";
import { PageHeader } from "../components/PageHeader";
import { ProjectNav } from "../features/project/ProjectNav";
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
  return (
    <>
      <PageHeader
        back={{ href: "#/", label: t("nav.backToProjects") }}
        eyebrow={title ? project.name : t("project.title")}
        title={title ?? project.name}
        subtitle={subtitle}
        actions={actions}
      />
      <ProjectNav projectId={project.id} current={current} runId={runId} />
      <div class={styles.stack}>{children}</div>
    </>
  );
}
