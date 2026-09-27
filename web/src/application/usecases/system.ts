import type { BuildInfo } from "../../domain/models";
import type { Ports } from "../ports";

const fallbackBuild: BuildInfo = { demoBuild: false, clientName: "" };

export function systemUseCases({ system }: Pick<Ports, "system">) {
  return {
    /** Build facts for the shell; defaults keep the UI usable when health is unreachable. */
    loadBuildInfo: async (): Promise<BuildInfo> => {
      try {
        return await system.health();
      } catch {
        return fallbackBuild;
      }
    },
  };
}
