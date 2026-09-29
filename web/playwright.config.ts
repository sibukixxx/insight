import { defineConfig, devices } from "@playwright/test";

// Real Go binaries, empty databases, no paid LLM (see e2e/serve.sh).
// E2E_PORT_BASE moves all seven ports so two checkouts can run at once.
const BASE = Number(process.env.E2E_PORT_BASE ?? 8811);
const DELIVERY = BASE;
const DEMO = BASE + 1;
const SCRIPTED_LLM = BASE + 2;
// The phone-viewport run gets its own empty database.
const DEMO_MOBILE = BASE + 3;
const SCRIPTED_LLM_MOBILE = BASE + 4;
// The Japanese sample-scenario journey gets its own empty database too.
const DEMO_JA = BASE + 5;
const SCRIPTED_LLM_JA = BASE + 6;

export default defineConfig({
  testDir: "e2e",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [["list"]],
  use: { launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } : {}, trace: "retain-on-failure", screenshot: "only-on-failure", acceptDownloads: true },
  projects: [
    { name: "delivery", testMatch: /delivery\.spec\.ts/, use: { ...devices["Desktop Chrome"], baseURL: `http://127.0.0.1:${DELIVERY}` } },
    ...["en", "ja"].flatMap((locale) => [false, true].map((mobile) => ({
      name: `delivery-ux-${mobile ? "mobile" : "desktop"}-${locale}`, testMatch: /ux\.spec\.ts/,
      use: { ...(mobile ? devices["Pixel 7"] : devices["Desktop Chrome"]), locale, baseURL: `http://127.0.0.1:${DELIVERY}` },
    }))),
    { name: "demo", testMatch: /demo\.spec\.ts/, use: { ...devices["Desktop Chrome"], baseURL: `http://127.0.0.1:${DEMO}` } },
    { name: "demo-mobile", testMatch: /demo\.spec\.ts/, grep: /@mobile/, use: { ...devices["Pixel 7"], baseURL: `http://127.0.0.1:${DEMO_MOBILE}` } },
    { name: "demo-ja", testMatch: /samples-ja\.spec\.ts/, grepInvert: /@mobile/, use: { ...devices["Desktop Chrome"], locale: "ja", baseURL: `http://127.0.0.1:${DEMO_JA}` } },
    { name: "demo-ja-mobile", testMatch: /samples-ja\.spec\.ts/, grep: /@mobile/, dependencies: ["demo-ja"], use: { ...devices["Pixel 7"], locale: "ja", baseURL: `http://127.0.0.1:${DEMO_JA}` } },
    // Question-first exploration (#158): model-backed on the demo/scripted server, model-less on delivery.
    // They run after the suites above so the shared databases still start empty for those.
    { name: "question-first", testMatch: /question-first\.spec\.ts/, grep: /@model/, dependencies: ["demo-ja-mobile"], use: { ...devices["Desktop Chrome"], locale: "ja", baseURL: `http://127.0.0.1:${DEMO_JA}` } },
    { name: "question-first-mobile", testMatch: /question-first\.spec\.ts/, grep: /@model/, dependencies: ["question-first"], use: { ...devices["Pixel 7"], locale: "ja", baseURL: `http://127.0.0.1:${DEMO_JA}` } },
    { name: "question-first-delivery", testMatch: /question-first\.spec\.ts/, grep: /@nomodel/, dependencies: ["delivery"], use: { ...devices["Desktop Chrome"], locale: "ja", baseURL: `http://127.0.0.1:${DELIVERY}` } },
  ],
  webServer: [
    { command: `sh e2e/serve.sh delivery ${DELIVERY}`, url: `http://127.0.0.1:${DELIVERY}/api/health`, timeout: 180_000, reuseExistingServer: false },
    { command: `sh e2e/serve.sh demo ${DEMO} ${SCRIPTED_LLM}`, url: `http://127.0.0.1:${DEMO}/api/health`, timeout: 180_000, reuseExistingServer: false },
    { command: `sh e2e/serve.sh demo ${DEMO_MOBILE} ${SCRIPTED_LLM_MOBILE}`, url: `http://127.0.0.1:${DEMO_MOBILE}/api/health`, timeout: 180_000, reuseExistingServer: false },
    { command: `sh e2e/serve.sh demo ${DEMO_JA} ${SCRIPTED_LLM_JA}`, url: `http://127.0.0.1:${DEMO_JA}/api/health`, timeout: 180_000, reuseExistingServer: false },
  ],
});
