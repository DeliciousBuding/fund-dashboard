import { useEffect, useRef, useCallback, useState } from 'react'

interface UseSSEOptions {
  /** Auto-reconnect delay in ms (default 5000) */
  reconnectMs?: number
}

interface UseSSEReturn {
  connected: boolean
  error: string | null
}

/**
 * useSSE — EventSource-based hook for Server-Sent Events.
 *
 * Connects to `url`, calls `onMessage` for each incoming event.
 * Auto-reconnects after `reconnectMs` (default 5s) on disconnect/error.
 * Cleans up on unmount.
 *
 * Returns { connected, error } for UI feedback.
 */
export function useSSE(
  url: string,
  onMessage: (event: MessageEvent) => void,
  options: UseSSEOptions = {},
): UseSSEReturn {
  const { reconnectMs = 5000 } = options
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const watchdogRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const esRef = useRef<EventSource | null>(null)
  // Exponential backoff bookkeeping: consecutive failures grow the reconnect
  // delay (base → 2× → 4× → 8× ... capped), reset to the base on a successful
  // onopen. Kept in a ref so it survives across reconnects without re-render.
  const consecutiveFailuresRef = useRef(0)
  const MAX_BACKOFF_MS = 60000

  const computeBackoff = useCallback(() => {
    const base = reconnectMs
    const n = consecutiveFailuresRef.current
    // base * 2^n, capped at MAX_BACKOFF_MS
    const raw = base * Math.pow(2, n)
    return Math.min(raw, MAX_BACKOFF_MS)
  }, [reconnectMs])

  const connect = useCallback(() => {
    // Clean up any existing connection
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }

    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => {
      // Successful open — reset backoff to the base delay.
      consecutiveFailuresRef.current = 0
      setConnected(true)
      setError(null)
      // Clear CONNECTING watchdog since we successfully connected.
      if (watchdogRef.current) {
        clearTimeout(watchdogRef.current)
        watchdogRef.current = null
      }
    }

    es.onmessage = (event) => {
      onMessageRef.current(event)
    }

    // EventSource dispatches named events via addEventListener,
    // but onmessage only fires for unnamed events. We also listen
    // for the "indices" named event.
    es.addEventListener('indices', (event: MessageEvent) => {
      onMessageRef.current(event)
    })

    es.onerror = () => {
      setConnected(false)
      // EventSource sets readyState to CLOSED on network error
      if (es.readyState === EventSource.CLOSED) {
        setError('Connection lost')
        es.close()
        esRef.current = null

        // Clear CONNECTING watchdog (CLOSED resolved, no need for watchdog).
        if (watchdogRef.current) {
          clearTimeout(watchdogRef.current)
          watchdogRef.current = null
        }

        // Exponential backoff: increase delay with each consecutive failure.
        const delay = computeBackoff()
        consecutiveFailuresRef.current += 1

        // Schedule reconnect
        if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = setTimeout(() => {
          connect()
        }, delay)
      } else {
        // CONNECTING — still trying, delegate to browser-native retry
        setError('Reconnecting...')

        // Watchdog: if EventSource stays stuck in CONNECTING, force-close
        // it after reconnectMs so the hook can take over with its own backoff
        // reconnect rather than waiting indefinitely on the browser.
        if (watchdogRef.current) clearTimeout(watchdogRef.current)
        watchdogRef.current = setTimeout(() => {
          watchdogRef.current = null
          es.close()
          // Invoke onerror — it will see CLOSED and trigger the backoff reconnect.
          if (es.onerror) es.onerror()
        }, reconnectMs)
      }
    }
  }, [url, reconnectMs, computeBackoff])

  useEffect(() => {
    connect()

    return () => {
      if (watchdogRef.current) clearTimeout(watchdogRef.current)
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      if (esRef.current) {
        esRef.current.close()
        esRef.current = null
      }
    }
  }, [connect])

  return { connected, error }
}
