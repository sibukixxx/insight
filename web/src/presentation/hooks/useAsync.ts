import { useCallback, useEffect, useRef, useState } from "preact/hooks";

export type AsyncState<T> =
  | { readonly status: "loading" }
  | { readonly status: "ok"; readonly data: T }
  | { readonly status: "error"; readonly error: unknown };

/**
 * Runs load whenever deps change and ignores results of superseded calls.
 * reload() re-runs it without clearing the current data first.
 */
export function useAsync<T>(load: () => Promise<T>, deps: readonly unknown[]): AsyncState<T> & { reload: () => void } {
  const [state, setState] = useState<AsyncState<T>>({ status: "loading" });
  const generation = useRef(0);
  const run = useCallback((keep: boolean) => {
    const current = ++generation.current;
    if (!keep) setState({ status: "loading" });
    load().then(
      (data) => { if (current === generation.current) setState({ status: "ok", data }); },
      (error: unknown) => { if (current === generation.current) setState({ status: "error", error }); },
    );
  }, deps);
  useEffect(() => {
    run(false);
    return () => { generation.current++; };
  }, [run]);
  const reload = useCallback(() => run(true), [run]);
  return { ...state, reload };
}
