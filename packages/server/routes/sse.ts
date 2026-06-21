/** /api/market/stream — SSE real-time index price push
 *
 *  Pushes index prices (^NDX/^GSPC/^DJI/^IXIC + CN/HK indices) every 60s.
 *  Reads from the indices table (populated by the Yahoo Finance crawler).
 *  Clients that disconnect are automatically cleaned up via onAbort.
 */

import { Hono } from "hono";
import { streamSSE } from "hono/streaming";
import { query } from "../db";
import { log } from "../middleware/logger";

interface IndexRow {
  code: string;
  name: string;
  market: string;
  price: number | null;
  change_pct: number | null;
  change_amt: number | null;
  updated_at: string;
}

const SSE_INTERVAL_MS = 60_000;

/** Track active SSE connections for observability */
let activeConnections = 0;

function getIndices(): IndexRow[] {
  return query<IndexRow>("SELECT * FROM indices ORDER BY code");
}

const router = new Hono();

router.get("/stream", (c) => {
  return streamSSE(c, async (stream) => {
    activeConnections++;
    log.info(`SSE client connected (total: ${activeConnections})`);

    let timer: ReturnType<typeof setInterval> | null = null;

    // Clean up on client disconnect
    stream.onAbort(() => {
      if (timer) clearInterval(timer);
      activeConnections--;
      log.info(`SSE client disconnected (total: ${activeConnections})`);
    });

    // Send initial data immediately
    try {
      const indices = getIndices();
      await stream.writeSSE({
        event: "indices",
        data: JSON.stringify(indices.map(r => ({
          code: r.code,
          name: r.name,
          market: r.market,
          price: r.price ? +r.price : null,
          change_pct: r.change_pct != null ? +r.change_pct : null,
          change_amt: r.change_amt != null ? +r.change_amt : null,
          updated_at: r.updated_at,
        }))),
      });
    } catch (e: any) {
      log.warn("SSE initial fetch failed", { error: e.message });
    }

    // Periodic push every 60s
    timer = setInterval(async () => {
      if (stream.aborted) return;
      try {
        const indices = getIndices();
        await stream.writeSSE({
          event: "indices",
          data: JSON.stringify(indices.map(r => ({
            code: r.code,
            name: r.name,
            market: r.market,
            price: r.price ? +r.price : null,
            change_pct: r.change_pct != null ? +r.change_pct : null,
            change_amt: r.change_amt != null ? +r.change_amt : null,
            updated_at: r.updated_at,
          }))),
        });
      } catch (e: any) {
        log.warn("SSE periodic fetch failed", { error: e.message });
      }
    }, SSE_INTERVAL_MS);

    // Keep stream alive — sleep in a loop until aborted
    while (!stream.aborted) {
      await stream.sleep(5000);
    }
  }, (err) => {
    // onError — stream-level error (separate from onAbort)
    if (err) log.warn(`SSE stream error: ${err.message}`);
  });
});

export default router;
