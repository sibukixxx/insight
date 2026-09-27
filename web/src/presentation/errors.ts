import { AppError } from "../application/errors";
import type { I18n } from "./i18n/I18nProvider";

/** Server messages are shown verbatim; transport failures get a localized explanation. */
export function errorMessage(error: unknown, t: I18n["t"]): string {
  if (error instanceof AppError) {
    if (error.kind === "network") return t("error.network", { message: error.message });
    if (error.kind === "invalid-response") return t("error.invalidResponse", { message: error.message });
    return error.message;
  }
  return error instanceof Error ? error.message : String(error);
}
