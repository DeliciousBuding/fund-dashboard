// 市场指数 SSE 流：/api/market/stream，event: indices。
// 设计纪律（03 §5）：SSE 推送的 ticker 更新无动画，防视觉噪音。
// 服务端有 max-lifetime，断线自动重连（指数退避上限 30s）。
// 合法帧同时回写 TanStack Query 缓存（["market-indices"]，与市场页
// useIndices 同一 key / 同一形状），市场页因此不再 60s 轮询：
// 首拉走 REST，后续更新由 SSE 帧驱动。

import { type MarketIndex, MarketIndexSchema } from "@fund-dashboard/contracts";
import type { QueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { MARKET_INDICES_QUERY_KEY } from "../lib/queries";
import { queryClient } from "../lib/queryClient";

interface MarketStreamState {
  indices: MarketIndex[];
  connected: boolean;
  updatedAt: string | null;
}

// applyIndicesFrame — zod 校验一帧 indices；合法则回写查询缓存并返回解析结果，
// 脏帧返回 null（整体跳过，等下一帧）。与 /api/market/indices 同一线型。
export function applyIndicesFrame(client: QueryClient, raw: unknown): MarketIndex[] | null {
  const parsed = z.array(MarketIndexSchema).safeParse(raw);
  if (!parsed.success) return null;
  client.setQueryData(MARKET_INDICES_QUERY_KEY, parsed.data);
  return parsed.data;
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
          const data = applyIndicesFrame(queryClient, JSON.parse(ev.data) as unknown);
          if (data == null) {
            // 脏帧跳过，等下一帧
            console.warn("[market-stream] invalid indices frame");
            return;
          }
          retryRef.current = 0;
          setState({
            indices: data,
            connected: true,
            updatedAt: new Date().toISOString(),
          });
        } catch (err) {
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
