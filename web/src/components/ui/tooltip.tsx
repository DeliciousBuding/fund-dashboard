import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { cn } from "../../lib/utils";

export const TooltipProvider = TooltipPrimitive.Provider;
export const Tooltip = TooltipPrimitive.Root;
export const TooltipTrigger = TooltipPrimitive.Trigger;

export function TooltipContent({ className, ...props }: TooltipPrimitive.TooltipContentProps) {
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        sideOffset={6}
        className={cn(
          "z-50 max-w-64 rounded-lg border border-border bg-surface-2 px-2.5 py-1.5 text-xs text-fg shadow-[0_8px_30px_rgb(0_0_0/0.35)]",
          className,
        )}
        {...props}
      />
    </TooltipPrimitive.Portal>
  );
}
