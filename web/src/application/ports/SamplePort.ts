import type { Project, SampleScenario } from "../../domain/models";
import type { UploadFile } from "./EvidencePort";

/** Bundled sample scenarios; a delivery build lists none. */
export interface SamplePort {
  list(): Promise<readonly SampleScenario[]>;
  /** Creates the scenario's empty project once and reuses it afterwards. */
  openProject(scenarioId: string): Promise<Project>;
  /** The scenario's input CSV, byte for byte, ready for the import preview. */
  input(scenarioId: string): Promise<UploadFile>;
}
