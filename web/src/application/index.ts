import type { Ports } from "./ports";
import { analysisUseCases } from "./usecases/analysis";
import { dashboardUseCases } from "./usecases/dashboard";
import { exportUseCases } from "./usecases/exportReport";
import { historyUseCases } from "./usecases/history";
import { importEvidenceUseCases } from "./usecases/importEvidence";
import { localeUseCases } from "./usecases/locale";
import { researchUseCases } from "./usecases/research";
import { resultsUseCases } from "./usecases/results";
import { settingsUseCases } from "./usecases/settings";
import { systemUseCases } from "./usecases/system";
import { workspaceUseCases } from "./usecases/workspace";

/** Everything the presentation layer may do, built from ports. */
export function createUseCases(ports: Ports) {
  return {
    system: systemUseCases(ports),
    dashboard: dashboardUseCases(ports),
    workspace: workspaceUseCases(ports),
    input: importEvidenceUseCases(ports),
    analysis: analysisUseCases(ports),
    results: resultsUseCases(ports),
    history: historyUseCases(ports),
    research: researchUseCases(ports),
    settings: settingsUseCases(ports),
    exports: exportUseCases(ports),
    locale: localeUseCases(ports),
  };
}

export type UseCases = ReturnType<typeof createUseCases>;
