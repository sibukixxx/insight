// Boundary validation for API JSON. Decoders accept exactly the shapes the
// Go handlers emit and keep "absent" (omitempty / null) distinct from zero,
// false or "": an absent optional field decodes to undefined.

export class DecodeError extends Error {
  constructor(path: string, expected: string, actual: unknown) {
    super(`unexpected API response at ${path}: expected ${expected}, got ${actual === null ? "null" : typeof actual}`);
    this.name = "DecodeError";
  }
}

export type Obj = Readonly<Record<string, unknown>>;

export function obj(v: unknown, path: string): Obj {
  if (typeof v !== "object" || v === null || Array.isArray(v)) throw new DecodeError(path, "object", v);
  return v as Obj;
}

export function str(o: Obj, key: string, path: string): string {
  const v = o[key];
  if (typeof v !== "string") throw new DecodeError(`${path}.${key}`, "string", v);
  return v;
}

/** A string field the server may omit; "" is kept as "", absence as undefined. */
export function optStr(o: Obj, key: string, path: string): string | undefined {
  const v = o[key];
  if (v === undefined || v === null) return undefined;
  if (typeof v !== "string") throw new DecodeError(`${path}.${key}`, "string", v);
  return v;
}

/** A string the server always sends but that may legitimately be empty. */
export function strOrEmpty(o: Obj, key: string, path: string): string {
  return optStr(o, key, path) ?? "";
}

export function num(o: Obj, key: string, path: string): number {
  const v = o[key];
  if (typeof v !== "number" || Number.isNaN(v)) throw new DecodeError(`${path}.${key}`, "number", v);
  return v;
}

export function optNum(o: Obj, key: string, path: string): number | undefined {
  const v = o[key];
  if (v === undefined) return undefined;
  if (typeof v !== "number" || Number.isNaN(v)) throw new DecodeError(`${path}.${key}`, "number", v);
  return v;
}

/** A number that may be JSON null (a metric the run did not record). */
export function numOrNull(o: Obj, key: string, path: string): number | null {
  const v = o[key];
  if (v === null || v === undefined) return null;
  if (typeof v !== "number" || Number.isNaN(v)) throw new DecodeError(`${path}.${key}`, "number or null", v);
  return v;
}

export function bool(o: Obj, key: string, path: string): boolean {
  const v = o[key];
  if (typeof v !== "boolean") throw new DecodeError(`${path}.${key}`, "boolean", v);
  return v;
}

export function arr<T>(v: unknown, path: string, item: (x: unknown, path: string) => T): T[] {
  if (!Array.isArray(v)) throw new DecodeError(path, "array", v);
  return v.map((x, i) => item(x, `${path}[${i}]`));
}

/** An array field the server may omit or send as null. */
export function optArr<T>(o: Obj, key: string, path: string, item: (x: unknown, path: string) => T): T[] {
  const v = o[key];
  if (v === undefined || v === null) return [];
  return arr(v, `${path}.${key}`, item);
}

export function stringItem(x: unknown, path: string): string {
  if (typeof x !== "string") throw new DecodeError(path, "string", x);
  return x;
}

export function stringRecord(v: unknown, path: string): Readonly<Record<string, string>> {
  if (v === undefined || v === null) return {};
  const o = obj(v, path);
  const out: Record<string, string> = {};
  for (const [k, value] of Object.entries(o)) out[k] = stringItem(value, `${path}.${k}`);
  return out;
}

export function numberRecord(v: unknown, path: string): Readonly<Record<string, number>> | undefined {
  if (v === undefined || v === null) return undefined;
  const o = obj(v, path);
  const out: Record<string, number> = {};
  for (const [k, value] of Object.entries(o)) {
    if (typeof value !== "number") throw new DecodeError(`${path}.${k}`, "number", value);
    out[k] = value;
  }
  return out;
}

export function boolRecord(v: unknown, path: string): Readonly<Record<string, boolean>> {
  if (v === undefined || v === null) return {};
  const o = obj(v, path);
  const out: Record<string, boolean> = {};
  for (const [k, value] of Object.entries(o)) out[k] = Boolean(value);
  return out;
}

type OptionalKeys<T> = { [K in keyof T]-?: object extends Pick<T, K> ? K : never }[keyof T];
/** T, except that optional properties may also be given as undefined. */
export type Loose<T> = Omit<T, OptionalKeys<T>> & { [K in OptionalKeys<T>]?: T[K] | undefined };

/** Drops undefined values so optional properties stay absent (exactOptionalPropertyTypes). */
export function compact<T extends object>(o: Loose<T>): T {
  return Object.fromEntries(Object.entries(o).filter(([, v]) => v !== undefined)) as T;
}
