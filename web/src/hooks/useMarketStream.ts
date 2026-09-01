// 市场指数 SSE 流：/api/market/stream，event: indices。
// 设计纪律（03 §5）：SSE 推送的 ticker 更新无动画，防视觉噪音。
// 服务端有 max-lifetime，断线自动重连（指数退避上限 30s）。

import type { MarketIndex } from "@fund-dashboard/contracts";
import { useEffect, useRef, useState } from "react";

export interface MarketStreamState {
  indices: MarketIndex[];
  connected: boolean;
  updatedAt: string | null;
}

export function useMarketStream(enabled = true): MarketStreamState {
  const [state, setState] = useState<MarketStreamState>({
    indices: [],
    connected: false,
    updatedAt: null,
  });
  const retryRef = useRef(0);

  useEffect(() => {
    if (!enabled) return;
    let closed = false;
    let source: EventSource | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      if (closed) return;
      source = new EventSource("/api/market/stream");
      source.addEventListener("indices", (ev) => {
        try {
          const data = JSON.parse((ev as MessageEvent).data) as MarketIndex[];
          retryRef.current = 0;
          setState({
            indices: data,
            connected: true,
            updatedAt: new Date().toISOString(),
          });
        } catch (err) {
          // 脏帧跳过，等下一帧
          console.warn("[market-stream] invalid indices frame", err);
        }
      });
      source.onerror = () => {
        setState((s) => ({ ...s, connected: false }));
        source?.close();
        if (closed) return;
        // 指数退避重连：1s → 2s → … → 30s 封顶
        const delay = Math.min(1000 * 2 ** retryRef.current, 30_000);
        retryRef.current += 1;
        timer = setTimeout(connect, delay);
      };
    };
    connect();

    return () => {
      closed = true;
      if (timer) clearTimeout(timer);
      source?.close();
    };
  }, [enabled]);

  return state;
}
