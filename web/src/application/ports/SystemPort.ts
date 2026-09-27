import type { BuildInfo, ImportFormat } from "../../domain/models";

export interface SystemPort {
  health(): Promise<BuildInfo>;
  /** Formats the import endpoints accept; the UI never offers others. */
  importFormats(): Promise<readonly ImportFormat[]>;
}
