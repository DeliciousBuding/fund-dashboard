import { useState, useEffect, useRef, useCallback } from 'react'
import { Text } from '@cloudflare/kumo'
import { TrendUp, CaretDown, ArrowClockwise } from '@phosphor-icons/react'
import { fetchIndices, type MarketIndex } from '../api'
import { useSSE } from '../hooks/useSSE'

/** Market-group definitions for ticker bar layout.
 *  Each group shows indices from a given market. */
const MARKET_GROUPS: Record<string, { label: string; codes: string[] }> = {
  us:  { label: '美股', codes: ['^IXIC', '^NDX', '^GSPC', '^DJI'] },
  cn:  { label: 'A股', codes: ['sh000001', 'sz399001', 'sz399006'] },
  hk:  { label: '港股', codes: ['^HSI'] },
}

/** Map a backend index code to a user-friendly short name */
function shortName(idx: MarketIndex): string {
  const nameMap: Record<string, string> = {
    '^IXIC': '纳指', '^NDX': '纳指100', '^GSPC': '标普500', '^DJI': '道指',
    'sh000001': '上证', 'sz399001': '深成指', 'sz399006': '创业板',
    '^HSI': '恒生',
  }
  return nameMap[idx.code] || idx.name
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
      const found = indices.find(i => i.code === code)
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
        padding: '6px 0', marginBottom: 16,
        borderBottom: '1px solid var(--color-kumo-border)',
        flexWrap: 'wrap',
        userSelect: 'none',
      }}
    >
      {/* SSE connection indicator dot + refresh button */}
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 2 }}>
        <span
          title={sseConnected ? '实时连接' : '轮询模式'}
          style={{
            display: 'inline-block', width: 6, height: 6, borderRadius: '50%',
            background: sseConnected ? '#199c63' : '#999',
            transition: 'background .3s',
          }}
        />
        <button
          onClick={refresh}
          disabled={refreshing}
          title="刷新行情"
          style={{
            background: 'none', border: 'none', cursor: 'pointer',
            padding: '2px 6px', marginRight: 4, borderRadius: 4,
            display: 'flex', alignItems: 'center',
            opacity: refreshing ? 0.4 : 0.55,
            transition: 'opacity .15s',
          }}
          onMouseEnter={e => { if (!refreshing) e.currentTarget.style.opacity = '1' }}
          onMouseLeave={e => { if (!refreshing) e.currentTarget.style.opacity = '0.55' }}
        >
          <ArrowClockwise size={14} style={{ animation: refreshing ? 'spin 1s linear infinite' : undefined }} />
        </button>
      </span>

      {/* Group labels rendered as subtle dividers when expanded */}
      {visible.map((o, i) => {
        const showGroup = expanded && groupBoundaries.includes(ordered.indexOf(o))
        return (
          <span key={o.idx.code} style={{ display: 'inline-flex', alignItems: 'center' }}>
            {showGroup && (
              <span style={{
                fontSize: 9, fontWeight: 700, color: 'var(--text-color-kumo-subtle)',
                marginRight: 8, marginLeft: 12, textTransform: 'uppercase',
                padding: '1px 6px', borderRadius: 3,
                background: 'var(--color-kumo-canvas)',
              }}>
                {o.group === 'us' ? 'US' : o.group === 'cn' ? 'CN' : o.group === 'hk' ? 'HK' : ''}
              </span>
            )}
            <TickerItem idx={o.idx} />
          </span>
        )
      })}

      {/* Expand/collapse toggle */}
      {ordered.length > 4 && (
        <button
          onClick={() => setExpanded(v => !v)}
          title={expanded ? '收起' : '展开所有指数'}
          style={{
            background: 'none', border: 'none', cursor: 'pointer',
            padding: '2px 6px', marginLeft: 0, borderRadius: 4,
            display: 'flex', alignItems: 'center', opacity: 0.5,
          }}
        >
          <CaretDown
            size={14}
            style={{
              transition: 'transform .2s',
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
  const isUp = (idx.change_pct ?? 0) >= 0
  const color = isUp ? '#d63649' : '#199c63'

  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: 4,
      fontSize: 13, fontWeight: 500, padding: '2px 8px', borderRadius: 6,
      transition: 'background .15s',
      cursor: 'default',
    }}
      onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-kumo-canvas)' }}
      onMouseLeave={e => { e.currentTarget.style.background = 'transparent' }}
    >
      <TrendUp size={16} weight="fill" style={{ color }} />
      <Text as="span" size="sm" bold>{shortName(idx)}</Text>
      <Text as="span" size="sm" style={{ fontVariantNumeric: 'tabular-nums' }}>
        {idx.price != null
          ? idx.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
          : '—'}
      </Text>
      <Text as="span" size="xs" style={{
        fontVariantNumeric: 'tabular-nums',
        color,
        fontWeight: 600,
        whiteSpace: 'nowrap',
      }}>
        {idx.change_pct != null
          ? `${isUp ? '+' : ''}${idx.change_pct.toFixed(2)}%`
          : '—'}
      </Text>
    </span>
  )
}
