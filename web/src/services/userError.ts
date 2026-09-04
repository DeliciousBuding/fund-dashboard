// 用户可见错误文案 —— sanitizeUserError（读路径/边界兜底）与
// mutationErrorMessage（写路径 toast/表单，#188）。

import { ZodError } from "zod";
import { ApiError } from "../lib/api";

/** Map raw Error/API messages to short user-facing copy (#188). */
export function sanitizeUserError(err: unknown, fallback: string): string {
  const raw = err instanceof Error ? err.message : String(err ?? "");
  const m = raw.trim();
  if (!m) return fallback;
  // Technical dumps / network noise — show fallback only
  if (
    /expected\s|received\s|at path|ECONNREFUSED|ENOTFOUND|network error|Failed to fetch|Load failed|HTTP\s*\d{3}|Zod|TypeError|SyntaxError|Internal Server Error/i.test(
      m,
    )
  ) {
    return fallback;
  }
  if (m.length > 160) return fallback;
  return m;
}

// 已知 API 错误码 → 中文文案（login/setup 原本地 errorText 并入，单一来源）。
const ERROR_CODE_TEXT: Record<string, string> = {
  invalid_credentials: "密码不正确",
  rate_limited: "尝试过于频繁，请稍后再试",
  not_initialized: "尚未初始化，请先设置密码",
  weak_password: "密码至少 10 个字符",
  already_initialized: "已初始化，请直接登录",
  auth_env_managed: "密码由部署环境变量管理，请直接登录",
};

// 机器味错误码：合成 http_* 与 snake_case 裸码一律不给用户看。
const MACHINE_CODE = /^(?:http_\d{3}|[a-z][a-z0-9]*(?:_[a-z0-9]+)+)$/;

/** Map a write-path (mutation) error to user-safe Chinese copy.
 *  ApiError 查码表给中文；ZodError / 网络错 / 机器码 → fallback，绝不明文 dump。 */
export function mutationErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ZodError) return fallback;
  if (err instanceof ApiError) {
    const known = ERROR_CODE_TEXT[err.code];
    if (known) return known;
    if (MACHINE_CODE.test(err.code)) return fallback;
    // 非机器码的 error 文案走通用净化（短句保留，技术噪音换 fallback）
    return sanitizeUserError(err, fallback);
  }
  return sanitizeUserError(err, fallback);
}
