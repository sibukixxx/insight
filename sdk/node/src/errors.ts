import type { ContractErrorCode } from "./contract.gen.ts";

/**
 * Client-side error codes. UNAVAILABLE means the engine could not be reached
 * or did not answer with a contract message. The other codes come from the
 * contract (x-errorCodes); UNSUPPORTED_CONTRACT_VERSION is also raised by the
 * client when a response uses a contract version this SDK does not speak.
 */
export type InsightErrorCode = ContractErrorCode | "UNAVAILABLE";

/** A typed error. httpStatus is 0 when the client raised it. */
export class InsightError extends Error {
  readonly code: InsightErrorCode;
  readonly httpStatus: number;

  constructor(code: InsightErrorCode, message: string, httpStatus = 0, options?: { cause?: unknown }) {
    super(message, options);
    this.name = "InsightError";
    this.code = code;
    this.httpStatus = httpStatus;
  }
}
