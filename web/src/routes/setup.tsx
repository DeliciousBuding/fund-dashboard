import { useMutation } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import type { ApiError } from "../lib/api";
import { setup } from "../lib/auth";
import { refreshAuthStatus } from "../lib/authQuery";
import { queryClient } from "../lib/queryClient";

const errorText: Record<string, string> = {
  weak_password: "密码至少 10 个字符",
  already_initialized: "已初始化，请直接登录",
  auth_env_managed: "密码由部署环境变量管理，请直接登录",
  rate_limited: "尝试过于频繁，请稍后再试",
};

export function SetupPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: setup,
    onSuccess: async () => {
      await refreshAuthStatus(queryClient);
      await router.navigate({ to: "/" });
    },
  });

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (password.length < 10) {
      setLocalError("密码至少 10 个字符");
      return;
    }
    if (password !== confirm) {
      setLocalError("两次输入不一致");
      return;
    }
    setLocalError(null);
    mutation.mutate(password);
  };

  const message =
    localError ??
    (mutation.error ? (errorText[(mutation.error as ApiError).code] ?? "初始化失败") : null);

  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 grid size-12 place-items-center rounded-2xl bg-surface-2 text-xl text-accent">
            ◈
          </div>
          <h1 className="text-xl font-medium text-fg">欢迎使用持仓中枢</h1>
          <p className="mt-1 text-sm text-fg-3">首次启动，请设置你的登录密码</p>
        </div>
        <form
          onSubmit={onSubmit}
          className="rounded-xl border border-border bg-surface-1 p-6 shadow-[0_8px_30px_rgb(0_0_0/0.35)]"
        >
          <label htmlFor="password" className="mb-2 block text-sm text-fg-2">
            设置密码
          </label>
          <input
            id="password"
            type="password"
            // biome-ignore lint/a11y/noAutofocus: 初始化页聚焦密码框是标准 UX（页面唯一意图）
            autoFocus
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2.5 text-fg placeholder:text-fg-3 focus:border-accent focus:outline-none"
            placeholder="至少 10 个字符"
          />
          <label htmlFor="confirm" className="mt-4 mb-2 block text-sm text-fg-2">
            确认密码
          </label>
          <input
            id="confirm"
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2.5 text-fg placeholder:text-fg-3 focus:border-accent focus:outline-none"
            placeholder="再输入一次"
          />
          {message ? (
            <p role="alert" className="mt-3 text-sm text-danger">
              {message}
            </p>
          ) : null}
          <button
            type="submit"
            disabled={!password || !confirm || mutation.isPending}
            className="mt-5 w-full rounded-lg bg-accent px-4 py-2.5 font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {mutation.isPending ? "初始化中…" : "完成初始化"}
          </button>
        </form>
        <p className="mt-4 text-center text-xs text-fg-3">
          密码以 argon2id 哈希存储；部署方亦可通过环境变量预置
        </p>
      </div>
    </main>
  );
}
