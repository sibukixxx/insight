import type { LlmSettings } from "../../domain/models";

export interface LlmSettingsInput {
  readonly baseUrl: string;
  readonly model: string;
  /** "" keeps the configured key. */
  readonly apiKey: string;
}

export interface SettingsPort {
  get(): Promise<LlmSettings>;
  update(input: LlmSettingsInput): Promise<LlmSettings>;
  /** Probes the configured provider; resolves to the execution mode code. */
  test(): Promise<string>;
}
