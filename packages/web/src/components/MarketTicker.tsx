import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Text } from '@cloudflare/kumo'
import { TrendUp, CaretDown, ArrowClockwise } from '@phosphor-icons/react'
import { fetchIndices, type MarketIndex } from '../api'
import { useSSE } from '../hooks/useSSE'
import { useAppStore } from '../stores/appStore'
import { getTheme, space, radius, fontSize, fontWeight, cssTransition, opacity, hitTarget } from '../styles/theme'

/** Market-group definitions for ticker bar layout.
 *  Each group shows indices from a given market. Labels via i18n market.group.* (#132). */
const MARKET_GROUPS: Record<string, { labelKey: string; codes: string[] }> = {
  us:  { labelKey: 'market.group.us', codes: ['^IXIC', '^NDX', '^GSPC', '^DJI'] },
  // Yahoo codes (#100/#101); SPA Eastmoney aliases still accepted via indexCodeMatch.
  cn:  { labelKey: 'market.group.cn', codes: ['000001.SS', '399001.SZ', '399006.SZ'] },
  hk:  { labelKey: 'market.group.hk', codes: ['^HSI'] },
}

/** Normalize SPA Eastmoney codes ↔ Yahoo symbols for matching (#101). */
const INDEX_CODE_ALIASES: Record<string, string[]> = {
  '000001.SS': ['000001.SS', 'sh000001', 'SH000001'],
  '399001.SZ': ['399001.SZ', 'sz399001', 'SZ399001'],
  '399006.SZ': ['399006.SZ', 'sz399006', 'SZ399006'],
  '^HSI': ['^HSI', 'HSI'],
  '^IXIC': ['^IXIC', 'IXIC'],
  '^NDX': ['^NDX', 'NDX'],
  '^GSPC': ['^GSPC', 'GSPC'],
  '^DJI': ['^DJI', 'DJI'],
}

function indexCodeMatch(backendCode: string, wanted: string): boolean {
  if (backendCode === wanted) return true
  const aliases = INDEX_CODE_ALIASES[wanted] ?? [wanted]
  const upper = backendCode.toUpperCase()
  return aliases.some((a) => a.toUpperCase() === upper)
}

/** Map a backend index code to a user-friendly short name (i18n market.index.*) */
function shortName(idx: MarketIndex, t: (key: string, opts?: { defaultValue?: string }) => string): string {
  return t(`market.index.${idx.code}`, { defaultValue: idx.name })
}

/** Parse SSE "indices" event data into MarketIndex[] */
function parseSSEIndices(data: string): MarketIndex[] | null {
  try {
    const parsed = JSON.parse(data)
    if (Array.isArray(parsed)) return parsed as MarketIndex[]
    return null
  } catch {
    return null
  }
}

/** Global market ticker bar. Displays live indices from US, CN, and HK markets.
 *  Prefers SSE real-time push (/api/market/stream).
 *  Falls back to HTTP polling (GET /api/market/indices every 60s) when SSE is unavailable. */
export default function MarketTicker() {
  const { t } = useTranslation()
  const dark = useAppStore((s) => s.dark)
  const theme = getTheme(dark)
  const [indices, setIndices] = useState<MarketIndex[]>([])
  const [expanded, setExpanded] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  // ── SSE real-time path ────────────────────────────────────────────
  const onSSEMessage = useCallback((event: MessageEvent) => {
    const data = parseSSEIndices(event.data)
    if (data && data.length > 0) {
      setIndices(data)
      setRefreshing(false)
    }
  }, [])

  const { connected: sseConnected } = useSSE('/api/market/stream', onSSEMessage)

  // ── HTTP polling fallback ─────────────────────────────────────────
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const loadHttp = useCallback(async () => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    try {
      const data = await fetchIndices(ctrl.signal)
      if (!ctrl.signal.aborted) setIndices(data)
    } catch { /* silent */ }
    finally { setRefreshing(false) }
  }, [])

  const refresh = useCallback(() => {
    setRefreshing(true)
    // If SSE is connected, just wait for the next push
    if (sseConnected) {
      setTimeout(() => setRefreshing(false), 2000)
      return
    }
    loadHttp()
  }, [sseConnected, loadHttp])

  // HTTP polling: only active when SSE is NOT connected
  useEffect(() => {
    if (sseConnected) {
      // SSE active — stop HTTP polling
      if (timerRef.current) clearInterval(timerRef.current)
      timerRef.current = null
      abortRef.current?.abort()
      return
    }

    // SSE not connected — start HTTP polling
    loadHttp()
    timerRef.current = setInterval(loadHttp, 60_000)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
      abortRef.current?.abort()
    }
  }, [sseConnected, loadHttp])

  if (!indices.length) return null

  // Build ordered list: group by market, prioritize US → CN → HK
  const ordered: { idx: MarketIndex; group: string }[] = []
  for (const [groupKey, groupDef] of Object.entries(MARKET_GROUPS)) {
    for (const code of groupDef.codes) {
      const found = indices.find(i => indexCodeMatch(i.code, code))
      if (found) ordered.push({ idx: found, group: groupKey })
    }
  }
  // Append any indices not in the predefined groups
  for (const idx of indices) {
    if (!ordered.some(o => o.idx.code === idx.code)) {
      ordered.push({ idx, group: '' })
    }
  }

  if (!ordered.length) return null

  // Default compact: first 4 indices
  const visible = expanded ? ordered : ordered.slice(0, 4)

  // Determine last-visible group for group separator rendering
  const groupBoundaries: number[] = []
  let lastGroup = ''
  ordered.forEach((o, i) => {
    if (o.group && o.group !== lastGroup) {
      groupBoundaries.push(i)
      lastGroup = o.group
    }
  })

  return (
    <div
      style={{
        display: 'flex', alignItems: 'center', gap: 0,
        padding: `${space[2] - 2}px 0`, marginBottom: space[4],
        borderBottom: '1px solid var(--color-kumo-border)',
        flexWrap: 'wrap',
        userSelect: 'none',
      }}
    >
      {/* SSE connection indicator dot + refresh button */}
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: space[1] / 2 }}>
        <span
          title={sseConnected ? t('market.realtime') : t('market.polling')}
          style={{
            display: 'inline-block', width: 6, height: 6, borderRadius: '50%',
            background: sseConnected ? theme.down : theme.textMuted,
            transition: cssTransition('background', { duration: 'slow' }),
          }}
        />
        <button
          type="button"
          onClick={refresh}
          disabled={refreshing}
          title={t('market.refresh')}
          aria-label={t('market.refresh')}
          style={{
            background: 'none', border: 'none', cursor: 'pointer',
            padding: `0 ${space[2] - 2}px`, marginRight: space[1], borderRadius: radius.sm - 2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            minWidth: hitTarget.min, minHeight: hitTarget.min,
            opacity: refreshing ? opacity.disabled : opacity.soft,
            transition: cssTransition('opacity', { duration: 'fast' }),
          }}
          onMouseEnter={e => { if (!refreshing) e.currentTarget.style.opacity = String(opacity.solid) }}
          onMouseLeave={e => { if (!refreshing) e.currentTarget.style.opacity = String(opacity.soft) }}
        >
          <ArrowClockwise size={14} className={refreshing ? 'fd-spin' : undefined} aria-hidden />
        </button>
      </span>

      {/* Group labels rendered as subtle dividers when expanded */}
      {visible.map((o, i) => {
        const showGroup = expanded && groupBoundaries.includes(ordered.indexOf(o))
        const groupLabelKey = o.group && MARKET_GROUPS[o.group]?.labelKey
        return (
          <span key={o.idx.code} style={{ display: 'inline-flex', alignItems: 'center' }}>
            {showGroup && groupLabelKey && (
              <span style={{
                fontSize: fontSize.xs, fontWeight: fontWeight.bold, color: 'var(--text-color-kumo-subtle)',
                marginRight: space[2], marginLeft: space[3], textTransform: 'uppercase',
                padding: `1px ${space[2] - 2}px`, borderRadius: radius.xs,
                background: 'var(--color-kumo-canvas)',
              }}>
                {t(groupLabelKey)}
              </span>
            )}
            <TickerItem idx={o.idx} />
          </span>
        )
      })}

      {/* Expand/collapse toggle */}
      {ordered.length > 4 && (
        <button
          type="button"
          onClick={() => setExpanded(v => !v)}
          title={expanded ? t('market.collapse') : t('market.expandAll')}
          aria-label={expanded ? t('market.collapse') : t('market.expandAll')}
          style={{
            background: 'none', border: 'none', cursor: 'pointer',
            padding: `0 ${space[2] - 2}px`, marginLeft: 0, borderRadius: radius.sm - 2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            minWidth: hitTarget.min, minHeight: hitTarget.min,
            opacity: opacity.muted,
          }}
        >
          <CaretDown
            size={14}
            style={{
              transition: cssTransition('transform'),
              transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)',
            }}
          />
        </button>
      )}
    </div>
  )
}

/** Single ticker item: name + price + change percentage */
function TickerItem({ idx }: { idx: MarketIndex }) {
  const { t } = useTranslation()
  const dark = useAppStore((s) => s.dark)
  const theme = getTheme(dark)
  const isUp = (idx.change_pct ?? 0) >= 0
  const color = isUp ? theme.up : theme.down

  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: space[1],
      fontSize: fontSize.base, fontWeight: fontWeight.medium, padding: `2px ${space[2]}px`, borderRadius: radius.sm,
      transition: cssTransition('background', { duration: 'fast' }),
      cursor: 'default',
    }}
      onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-kumo-canvas)' }}
      onMouseLeave={e => { e.currentTarget.style.background = 'transparent' }}
    >
      <TrendUp size={16} weight="fill" style={{ color }} />
      <Text as="span" size="sm" bold>{shortName(idx, t)}</Text>
      <Text as="span" size="sm" style={{ fontVariantNumeric: 'tabular-nums' }}>
        {idx.price != null
          ? idx.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
          : '—'}
      </Text>
      <Text as="span" size="xs" style={{
        fontVariantNumeric: 'tabular-nums',
        color,
        fontWeight: fontWeight.semibold,
        whiteSpace: 'nowrap',
      }}>
        {idx.change_pct != null
          ? `${isUp ? '+' : ''}${idx.change_pct.toFixed(2)}%`
          : '—'}
      </Text>
    </span>
  )
}
