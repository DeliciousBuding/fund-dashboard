/**
 * Portfolio Service — source events (V4).
 *
 * Extracted from services/portfolio.ts (2026-06-19).
 */

import { query, queryOne, getRwDb } from "../db";
import type { SourceEventRow, CreateSourceEventInput, GetSourceEventsOptions } from "../utils/types";

export function createSourceEvent(input: CreateSourceEventInput): SourceEventRow {
  const db = getRwDb();
  const fetchedAt = new Date().toISOString().replace("T", " ").substring(0, 19);
  const result = db.run(
    `INSERT INTO source_events (title, url, source, snippet, query, related_security_code, related_security_name, fetched_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      input.title,
      input.url || null,
      input.source || "websearch",
      input.snippet || null,
      input.query || null,
      input.related_security_code || null,
      input.related_security_name || null,
      fetchedAt,
    ],
  );
  const id = Number(result.lastInsertRowid);
  return queryOne<SourceEventRow>("SELECT * FROM source_events WHERE id = ?", id)!;
}

export function getSourceEvents(opts: GetSourceEventsOptions = {}): SourceEventRow[] {
  const conditions: string[] = [];
  const params: any[] = [];
  const limit = Math.min(opts.limit || 30, 100);

  if (opts.related_security_code) {
    conditions.push("related_security_code = ?");
    params.push(opts.related_security_code);
  }
  if (opts.source) {
    conditions.push("source = ?");
    params.push(opts.source);
  }
  if (!opts.show_read) {
    conditions.push("is_read = ?");
    params.push(opts.is_read !== undefined ? opts.is_read : 0);
  }
  if (opts.is_read !== undefined && opts.show_read) {
    conditions.push("is_read = ?");
    params.push(opts.is_read);
  }

  const where = conditions.length ? `WHERE ${conditions.join(" AND ")}` : "";
  return query<SourceEventRow>(
    `SELECT * FROM source_events ${where} ORDER BY fetched_at DESC LIMIT ? OFFSET ?`,
    ...params, limit, opts.offset || 0,
  );
}

export function markSourceEventRead(id: number, fields: { is_read?: boolean; is_useful?: boolean }): boolean {
  const db = getRwDb();
  const sets: string[] = [];
  const vals: any[] = [];
  if (fields.is_read !== undefined) { sets.push("is_read = ?"); vals.push(fields.is_read ? 1 : 0); }
  if (fields.is_useful !== undefined) { sets.push("is_useful = ?"); vals.push(fields.is_useful ? 1 : 0); }
  if (!sets.length) return false;
  vals.push(id);
  const result = db.run(`UPDATE source_events SET ${sets.join(", ")} WHERE id = ?`, vals);
  return result.changes > 0;
}
