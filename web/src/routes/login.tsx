import { useMutation } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { login } from "../lib/auth";
import { refreshAuthStatus } from "../lib/authQuery";
import { oauthReturnTarget } from "../lib/oauthReturn";
import { queryClient } from "../lib/queryClient";
import { mutationErrorMessage } from "../services/userError";

export function LoginPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: async () => {
      await refreshAuthStatus(queryClient);
      // An OAuth authorize request that found no dashboard session is bounced
      // here with ?next=/oauth/authorize?... — finishing the login must resume
      // that authorization instead of dumping the owner on the overview page.
      // The target is a backend route the SPA router does not own, so this is a
      // real navigation, and it is validated before use (lib/oauthReturn).
      const next = oauthReturnTarget();
      if (next) {
        window.location.assign(next);
        return;
      }
      await router.navigate({ to: "/" });
    },
  });

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (!password) return;
    mutation.mutate(password);
  };

  const message = mutation.error
    ? mutationErrorMessage(mutation.error, "登录失败，请稍后重试")
    : null;

  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 grid size-12 place-items-center rounded-2xl bg-surface-2 text-xl text-accent">
            ◈
          </div>
          <h1 className="text-xl font-medium text-fg">持仓中枢</h1>
          <p className="mt-1 text-sm text-fg-3">输入密码进入你的投资组合</p>
        </div>
        <form
          onSubmit={onSubmit}
          className="rounded-xl border border-border bg-surface-1 p-6 shadow-[0_8px_30px_rgb(0_0_0/0.35)]"
        >
          <label htmlFor="password" className="mb-2 block text-sm text-fg-2">
            密码
          </label>
          <input
            id="password"
            type="password"
            // biome-ignore lint/a11y/noAutofocus: 登录页聚焦密码框是标准登录 UX（页面唯一意图）
            autoFocus
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2.5 text-fg placeholder:text-fg-3 focus:border-accent focus:outline-none"
            placeholder="••••••••••"
          />
          {message ? (
            <p role="alert" className="mt-3 text-sm text-danger">
              {message}
            </p>
          ) : null}
          <button
            type="submit"
            disabled={!password || mutation.isPending}
            className="mt-5 w-full rounded-lg bg-accent px-4 py-2.5 font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {mutation.isPending ? "登录中…" : "登录"}
          </button>
        </form>
      </div>
    </main>
  );
}
