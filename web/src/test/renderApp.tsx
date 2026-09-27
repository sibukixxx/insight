import { render } from "@testing-library/preact";
import { App } from "../app/App";
import { createCompositionRoot } from "../app/composition-root";
import type { Ports } from "../application/ports";
import type { Locale } from "../domain/locale";
import type { BuildInfo } from "../domain/models";
import { dictionaries } from "./fakePorts";

export function renderApp(ports: Ports, { hash = "#/", locale = "en", build = { demoBuild: false, clientName: "" } }: { hash?: string; locale?: Locale; build?: BuildInfo } = {}) {
  window.location.hash = hash;
  return render(<App useCases={createCompositionRoot(ports)} build={build} dictionaries={dictionaries} locale={locale} />);
}
