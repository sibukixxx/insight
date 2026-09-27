// Failures an adapter reports through a port. The server's own message is
// kept verbatim (it is not translated); only transport failures without a
// server message get a localized explanation in the presentation layer.

export type AppErrorKind = "http" | "network" | "invalid-response" | "unavailable";

export class AppError extends Error {
  readonly kind: AppErrorKind;
  readonly status: number | undefined;

  constructor(kind: AppErrorKind, message: string, status?: number) {
    super(message);
    this.name = "AppError";
    this.kind = kind;
    this.status = status;
  }
}

export function toAppError(error: unknown): AppError {
  if (error instanceof AppError) return error;
  return new AppError("network", error instanceof Error ? error.message : String(error));
}
