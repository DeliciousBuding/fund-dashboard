import { useQueries } from "@tanstack/react-query";
import { useState } from "react";
import { Badge } from "../../components/ui/badge";
import { Input } from "../../components/ui/input";
import { api } from "../../lib/api";
import { useSecurities } from "../../lib/queries";
import { cn } from "../../lib/utils";
import { useUi } from "../../stores/ui";

export const MAX_COMPARE = 8;

// ── 标的多选（compare/backtest/advanced 共用）─────────────────────────

export function CodePicker(props: {
  selected: string[];
  onChange: (codes: string[]) => void;
  max?: number;
}) {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const [query, setQuery] = useState("");

  const list = (securities.data ?? []).filter((s) => {
    if (!query) return true;
    const q = query.toLowerCase();
    return s.code.toLowerCase().includes(q) || s.name.toLowerCase().includes(q);
  });

  return (
    <div className="space-y-2">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="搜索标的加入对比…"
        aria-label="搜索标的"
      />
      <div className="flex flex-wrap gap-1.5">
        {props.selected.map((code) => {
          const sec = (securities.data ?? []).find((s) => s.code === code);
          return (
            <Badge
              key={code}
              tone="accent"
              className="cursor-pointer"
              onClick={() => props.onChange(props.selected.filter((c) => c !== code))}
            >
              {sec?.name ?? code} ✕
            </Badge>
          );
        })}
      </div>
      <div className="max-h-44 overflow-y-auto rounded-lg border border-border">
        {list.map((s) => {
          const on = props.selected.includes(s.code);
          return (
            <button
              key={s.code}
              type="button"
              disabled={!on && props.selected.length >= (props.max ?? MAX_COMPARE)}
              onClick={() =>
                props.onChange(
                  on ? props.selected.filter((c) => c !== s.code) : [...props.selected, s.code],
                )
              }
              className={cn(
                "flex w-full items-center justify-between px-3 py-1.5 text-left text-xs transition-colors",
                on ? "bg-accent/10 text-accent" : "text-fg-2 hover:bg-surface-3",
                !on &&
                  props.selected.length >= (props.max ?? MAX_COMPARE) &&
                  "cursor-not-allowed opacity-40",
              )}
            >
              <span className="truncate">{s.name}</span>
              <span className="ml-2 shrink-0 font-mono text-fg-3">{s.code}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// 多标的 NAV 并行拉取
export function useMultiNav(codes: string[]) {
  return useQueries({
    queries: codes.map((code) => ({
      queryKey: ["nav", code, 2000],
      queryFn: ({ signal }: { signal?: AbortSignal }) =>
        api<{ date: string; unit_nav: number }[]>(
          `/api/funds/${encodeURIComponent(code)}/nav?limit=2000`,
          { signal },
        ),
      staleTime: 5 * 60 * 1000,
    })),
  });
}
