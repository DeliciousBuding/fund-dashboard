// ═══════ Formatting helpers ═══════
// Extracted from utils.ts — pure formatting functions.

export function fmt(v: number): string {
  if (Math.abs(v) < 0.01) return '¥ 0.00';
  return v > 0 ? `+¥ ${v.toFixed(2)}` : `-¥ ${Math.abs(v).toFixed(2)}`;
}

export function fmtShort(v: number): string {
  const r = Math.round(v);
  if (r === 0) return '0';
  return r > 0 ? `+${r}` : `${r}`;
}

/** Format YYYY-MM-DD (or ISO) for UI by i18n language (#191). */
export function formatNavDate(iso: string | null | undefined, language: string): string {
  if (!iso) return ''
  const raw = iso.length >= 10 ? iso.slice(0, 10) : iso
  const d = new Date(`${raw}T00:00:00`)
  if (Number.isNaN(d.getTime())) return raw
  const locale = language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
  return d.toLocaleDateString(locale, { year: 'numeric', month: 'short', day: 'numeric' })
}
