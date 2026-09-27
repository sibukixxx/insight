import { AnalysisPage } from "../pages/AnalysisPage";
import { EvaluationPage } from "../pages/EvaluationPage";
import { FindingsPage } from "../pages/FindingsPage";
import { HomePage } from "../pages/HomePage";
import { InputPage } from "../pages/InputPage";
import { InsightPage } from "../pages/InsightPage";
import { PatternsPage } from "../pages/PatternsPage";
import { PromotionPage } from "../pages/PromotionPage";
import { ResearchPage } from "../pages/ResearchPage";
import { RunsPage } from "../pages/RunsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { WorkspacePage } from "../pages/WorkspacePage";
import type { Route } from "./routes";

export function Router({ route }: { route: Route }) {
  switch (route.name) {
    case "home": return <HomePage />;
    case "settings": return <SettingsPage />;
    case "workspace": return <WorkspacePage projectId={route.projectId} runId={route.runId} />;
    case "input": return <InputPage projectId={route.projectId} />;
    case "analysis": return <AnalysisPage projectId={route.projectId} />;
    case "findings": return <FindingsPage projectId={route.projectId} runId={route.runId} />;
    case "patterns": return <PatternsPage projectId={route.projectId} runId={route.runId} />;
    case "evaluation": return <EvaluationPage projectId={route.projectId} runId={route.runId} />;
    case "runs": return <RunsPage projectId={route.projectId} from={route.from} to={route.to} />;
    case "research": return <ResearchPage projectId={route.projectId} />;
    case "promotion": return <PromotionPage researchRunId={route.researchRunId} />;
    case "insight": return <InsightPage insightId={route.insightId} />;
  }
}
