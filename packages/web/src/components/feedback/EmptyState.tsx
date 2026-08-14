// Shared empty-state for lists / search / zero portfolio.
import type { ReactNode } from "react";
import { Text, Button } from "@cloudflare/kumo";
import { useAppStore } from "../../stores/appStore";
import { fontSize, getTheme, space } from "../../styles/theme";
import { Card } from "../ui/Card";

interface EmptyStateProps {
  title: string;
  description?: string;
  icon?: ReactNode;
  actionLabel?: string;
  onAction?: () => void;
  compact?: boolean;
  className?: string;
}

export default function EmptyState({
  title,
  description,
  icon,
  actionLabel,
  onAction,
  compact = false,
  className,
}: EmptyStateProps) {
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);

  const body = (
    <div
      className={className}
      role="status"
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        textAlign: "center",
        gap: space[2],
        padding: compact ? `${space[3]}px ${space[2]}px` : `${space[5]}px ${space[4]}px`,
        color: theme.textSubtle,
      }}
    >
      {icon && (
        <div aria-hidden style={{ color: theme.textMuted, marginBottom: space[1] }}>
          {icon}
        </div>
      )}
      <Text variant="body" as="span" style={{ fontSize: fontSize.base, color: theme.text, fontWeight: 500 }}>
        {title}
      </Text>
      {description && (
        <Text variant="secondary" as="span" size="sm" style={{ maxWidth: 280 }}>
          {description}
        </Text>
      )}
      {actionLabel && onAction && (
        <Button type="button" variant="secondary" size="sm" onClick={onAction} style={{ marginTop: space[2] }}>
          {actionLabel}
        </Button>
      )}
    </div>
  );

  if (compact) return body;
  return (
    <Card dark={dark} glass material="ultraThin" padded={false}>
      {body}
    </Card>
  );
}
