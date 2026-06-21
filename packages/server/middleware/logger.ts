/** Structured logging — agent-friendly, request-scoped */

const ENABLED = process.env.LOG_LEVEL !== "off";

function ts() { return new Date().toISOString().substring(11, 23); }

export interface LogCtx { reqId?: string; duration?: number; [k: string]: any }

function fmt(ctx: LogCtx, msg: string, level: string) {
  const { reqId, duration, ...rest } = ctx;
  const parts = [`${ts()} [${level}]`];
  if (reqId) parts.push(`req=${reqId}`);
  if (duration !== undefined) parts.push(`${duration}ms`);
  parts.push(msg);
  if (Object.keys(rest).length) parts.push(JSON.stringify(rest));
  return parts.join(" ");
}

export const log = {
  info(msg: string, ctx: LogCtx = {}) { if (ENABLED) console.log(fmt(ctx, msg, "INFO")); },
  warn(msg: string, ctx: LogCtx = {}) { console.warn(fmt(ctx, msg, "WARN")); },
  error(msg: string, ctx: LogCtx = {}) { console.error(fmt(ctx, msg, "ERROR")); },
  debug(msg: string, ctx: LogCtx = {}) { if (ENABLED || process.env.LOG_LEVEL === "debug") console.log(fmt(ctx, msg, "DEBUG")); },
};

/** Hono request logger middleware */
export function reqLogger(req: Request, reqId: string) {
  const start = Date.now();
  return {
    reqId,
    done(status: number) {
      const dur = Date.now() - start;
      const url = new URL(req.url);
      log.info(`${req.method} ${url.pathname} → ${status}`, { reqId, duration: dur, method: req.method, path: url.pathname });
    },
  };
}
