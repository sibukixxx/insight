import type { AnalysisPort, AnalysisStreamPort } from "./AnalysisPort";
import type { EvidencePort } from "./EvidencePort";
import type { LinkPort } from "./LinkPort";
import type { LocalePort } from "./LocalePort";
import type { ProjectPort } from "./ProjectPort";
import type { ResearchPort } from "./ResearchPort";
import type { ResultsPort } from "./ResultsPort";
import type { SamplePort } from "./SamplePort";
import type { SettingsPort } from "./SettingsPort";
import type { SystemPort } from "./SystemPort";

/** Every port the use cases need; wired once in app/composition-root.ts. */
export interface Ports {
  readonly system: SystemPort;
  readonly projects: ProjectPort;
  readonly evidence: EvidencePort;
  readonly analysis: AnalysisPort;
  readonly stream: AnalysisStreamPort;
  readonly results: ResultsPort;
  readonly research: ResearchPort;
  readonly settings: SettingsPort;
  readonly links: LinkPort;
  readonly locale: LocalePort;
  readonly samples: SamplePort;
}

export type { AnalysisPort, AnalysisStreamHandlers, AnalysisStreamPort, StartAnalysisInput } from "./AnalysisPort";
export type { EvidencePort, TextEvidenceInput, UploadFile } from "./EvidencePort";
export type { LinkPort } from "./LinkPort";
export type { LocalePort } from "./LocalePort";
export type { ProjectPort } from "./ProjectPort";
export type { ResearchPort } from "./ResearchPort";
export type { ResultsPort } from "./ResultsPort";
export type { SamplePort } from "./SamplePort";
export type { LlmSettingsInput, SettingsPort } from "./SettingsPort";
export type { SystemPort } from "./SystemPort";
