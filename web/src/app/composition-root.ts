// The single place where infrastructure adapters are bound to application
// ports. Nothing else imports infrastructure/.

import { createUseCases, type UseCases } from "../application";
import type { Ports } from "../application/ports";
import {
  httpAnalysis, httpEvidence, httpIngest, httpLinks, httpProjects, httpResearch, httpResults, httpSamples, httpSettings, httpSystem,
} from "../infrastructure/http/adapters";
import { createHttpClient } from "../infrastructure/http/client";
import { browserLocaleStore } from "../infrastructure/i18n/localeStore";
import { sseAnalysisStream } from "../infrastructure/sse/analysisEvents";

export function createPorts(): Ports {
  const http = createHttpClient();
  return {
    system: httpSystem(http),
    projects: httpProjects(http),
    evidence: httpEvidence(http),
    ingest: httpIngest(http),
    analysis: httpAnalysis(http),
    stream: sseAnalysisStream(),
    results: httpResults(http),
    research: httpResearch(http),
    settings: httpSettings(http),
    links: httpLinks,
    locale: browserLocaleStore(),
    samples: httpSamples(http),
  };
}

export function createCompositionRoot(ports: Ports = createPorts()): UseCases {
  return createUseCases(ports);
}
