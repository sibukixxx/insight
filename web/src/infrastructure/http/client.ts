// The only place the Reference Web talks HTTP. Server errors keep the
// server's message verbatim ({"error": "..."}); transport failures and
// malformed JSON become typed AppErrors.

import { AppError } from "../../application/errors";
import { DecodeError } from "./decode";

export interface HttpClient {
  getJson<T>(path: string, decode: (body: unknown) => T): Promise<T>;
  sendJson<T>(method: "POST" | "PUT" | "DELETE", path: string, body: unknown, decode: (body: unknown) => T): Promise<T>;
  sendForm<T>(path: string, form: FormData, decode: (body: unknown) => T): Promise<T>;
  /** A non-JSON download (e.g. a CSV) as a Blob; errors as for JSON. */
  getBlob(path: string): Promise<Blob>;
}

export type Fetch = (input: string, init?: RequestInit) => Promise<Response>;

export function createHttpClient(fetchImpl: Fetch = (input, init) => fetch(input, init)): HttpClient {
  async function request<T>(path: string, init: RequestInit, decode: (body: unknown) => T): Promise<T> {
    let res: Response;
    try {
      res = await fetchImpl(path, init);
    } catch (e) {
      throw new AppError("network", e instanceof Error ? e.message : String(e));
    }
    let body: unknown = null;
    const text = await res.text();
    if (text !== "") {
      try {
        body = JSON.parse(text);
      } catch {
        if (res.ok) throw new AppError("invalid-response", `${path}: response is not JSON`, res.status);
      }
    }
    if (!res.ok) {
      const message = typeof body === "object" && body !== null && typeof (body as { error?: unknown }).error === "string"
        ? (body as { error: string }).error
        : `${res.status} ${res.statusText}`.trim();
      throw new AppError("http", message, res.status);
    }
    try {
      return decode(body);
    } catch (e) {
      if (e instanceof DecodeError) throw new AppError("invalid-response", e.message, res.status);
      throw e;
    }
  }
  const json = { "Content-Type": "application/json" };
  return {
    getJson: (path, decode) => request(path, { headers: { Accept: "application/json" } }, decode),
    sendJson: (method, path, body, decode) =>
      request(path, { method, headers: json, body: body === undefined ? null : JSON.stringify(body) }, decode),
    sendForm: (path, form, decode) => request(path, { method: "POST", body: form }, decode),
    getBlob: async (path) => {
      let res: Response;
      try {
        res = await fetchImpl(path, {});
      } catch (e) {
        throw new AppError("network", e instanceof Error ? e.message : String(e));
      }
      if (res.ok) return res.blob();
      // Reuse the JSON path for the server's {"error": ...} message.
      const text = await res.text();
      let message = `${res.status} ${res.statusText}`.trim();
      try {
        const body = JSON.parse(text) as { error?: unknown };
        if (typeof body.error === "string") message = body.error;
      } catch { /* keep the status line */ }
      throw new AppError("http", message, res.status);
    },
  };
}

export const enc = encodeURIComponent;
export const ignoreBody = (): void => undefined;
