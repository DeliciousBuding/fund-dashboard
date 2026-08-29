import type * as React from "react";
import { cn } from "../../lib/utils";

export function Input({ className, ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "h-9 w-full rounded-lg border border-border bg-surface-2 px-3 text-sm text-fg placeholder:text-fg-3 focus:border-accent focus:outline-none disabled:opacity-40",
        className,
      )}
      {...props}
    />
  );
}

export function Label({ className, ...props }: React.LabelHTMLAttributes<HTMLLabelElement>) {
  // 原语层：关联（htmlFor / aria-label）由消费方负责。
  // biome-ignore lint/a11y/noLabelWithoutControl: primitive; consumers associate via htmlFor
  return <label className={cn("mb-1.5 block text-sm text-fg-2", className)} {...props} />;
}
