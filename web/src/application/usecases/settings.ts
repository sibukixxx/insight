import type { LlmSettings } from "../../domain/models";
import type { LlmSettingsInput, Ports } from "../ports";

export function settingsUseCases({ settings }: Pick<Ports, "settings">) {
  return {
    loadSettings: (): Promise<LlmSettings> => settings.get(),
    saveSettings: (input: LlmSettingsInput): Promise<LlmSettings> => settings.update(input),
    testConnection: (): Promise<string> => settings.test(),
  };
}
