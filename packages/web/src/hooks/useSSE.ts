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
  const esRef = useRef<EventSource | null>(null)

  const connect = useCallback(() => {
    // Clean up any existing connection
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }

    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => {
      setConnected(true)
      setError(null)
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

        // Schedule reconnect
        if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = setTimeout(() => {
          connect()
        }, reconnectMs)
      } else {
        // CONNECTING — still trying
        setError('Reconnecting...')
      }
    }
  }, [url, reconnectMs])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      if (esRef.current) {
        esRef.current.close()
        esRef.current = null
      }
    }
  }, [connect])

  return { connected, error }
}
