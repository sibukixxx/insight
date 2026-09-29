import { useEffect, useState } from "preact/hooks";
import type { AnalysisEvent } from "../../../domain/models";
import { useUseCases } from "../../services/context";
import type { LiveProgress } from "../runs/RunStatus";

/**
 * Follows runId while it is active. onFinished runs once when it completes
 * or fails; live holds the latest progress until then. unknown is true
 * while the run's state cannot be read (the connection is down).
 */
export function useRunWatcher(runId: string | undefined, onFinished: (event: Exclude<AnalysisEvent, LiveProgress>) => void) {
  const { analysis } = useUseCases();
  const [live, setLive] = useState<LiveProgress | undefined>(undefined);
  const [unknown, setUnknown] = useState(false);
  useEffect(() => {
    setLive(undefined);
    setUnknown(false);
    if (!runId) return undefined;
    return analysis.watchRun(runId, (event) => {
      if (event.type === "progress") setLive(event);
      else {
        setLive(undefined);
        onFinished(event);
      }
    }, setUnknown);
    // onFinished is intentionally not a dependency: the watcher must not restart on re-render.
  }, [runId, analysis]);
  return { live, unknown };
}
