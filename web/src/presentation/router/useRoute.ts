import { useEffect, useState } from "preact/hooks";
import { buildHash, parseHash, type Route } from "./routes";

export function useRoute(): Route {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash));
  useEffect(() => {
    const onChange = () => setRoute(parseHash(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return route;
}

export function navigate(route: Route | string): void {
  const hash = typeof route === "string" ? route : buildHash(route);
  if (window.location.hash !== hash) window.location.hash = hash;
}
