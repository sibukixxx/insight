import { createContext, type ComponentChildren } from "preact";
import { useContext } from "preact/hooks";
import type { UseCases } from "../../application";
import type { BuildInfo } from "../../domain/models";

const UseCasesContext = createContext<UseCases | undefined>(undefined);
const BuildContext = createContext<BuildInfo>({ demoBuild: false, clientName: "" });

export function ServicesProvider({ useCases, build, children }: { useCases: UseCases; build: BuildInfo; children: ComponentChildren }) {
  return (
    <UseCasesContext.Provider value={useCases}>
      <BuildContext.Provider value={build}>{children}</BuildContext.Provider>
    </UseCasesContext.Provider>
  );
}

export function useUseCases(): UseCases {
  const value = useContext(UseCasesContext);
  if (!value) throw new Error("useUseCases outside ServicesProvider");
  return value;
}

export function useBuild(): BuildInfo {
  return useContext(BuildContext);
}
