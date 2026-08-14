// Compact glass freshness control — last NAV + manual refetch.
// Severity tint uses Asia/Shanghai-aligned stale_nav_days from the portfolio API.
import { useTranslation } from "react-i18next";
import { Button } from "@cloudflare/kumo";
import { ArrowsClockwise, Clock, WarningCircle } from "@phosphor-icons/react";
import { useAppStore } from "../stores/appStore";
import { formatNavDate } from "../services/format";
import {
  fontSize,
  fontWeight,
  getTheme,
  glassSurfaceStyle,
  radius,
  space,
} from "../styles/theme";

interface Props {
  lastNavDate?: string | null;
  /** Whole calendar days since last NAV (CST), from portfolio.stale_nav_days. */
  staleDays?: number | null;
  isFetching?: boolean;
  onRefresh: () => void;
}

type StaleLevel = "ok" | "warn" | "critical" | "unknown";

function staleLevel(staleDays: number | null | undefined, hasDate: boolean): StaleLevel {
  if (!hasDate) return "unknown";
  if (staleDays == null || staleDays <= 2) return "ok";
  if (staleDays <= 7) return "warn";
  return "critical";
}

export default function DataFreshnessBar({
  lastNavDate,
  staleDays = null,
  isFetching = false,
  onRefresh,
}: Props) {
  const { t, i18n } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const offline = typeof navigator !== "undefined" && !navigator.onLine;
  const level = staleLevel(staleDays, Boolean(lastNavDate));

  const borderColor =
    level === "critical"
      ? theme.critical
      : level === "warn"
        ? theme.amber
        : undefined;
  const textColor =
    level === "critical"
      ? theme.critical
      : level === "warn"
        ? theme.amber
        : theme.textSubtle;

  let label: string;
  if (!lastNavDate) {
    label = t("stat.navPending");
  } else {
    const datePart = `${t("tx.navUpdated")} ${formatNavDate(lastNavDate, i18n.language)}`;
    if (staleDays != null && staleDays > 0) {
      label = `${datePart} ${t("tx.daysAgo", { days: staleDays })}`;
    } else if (staleDays === 0 || staleDays == null) {
      // null with a date means backend treats it as today (StaleNAVDays only set when > 0)
      label = `${datePart} ${t("tx.today")}`;
    } else {
      label = datePart;
    }
  }

  const Icon = level === "warn" || level === "critical" ? WarningCircle : Clock;

  return (
    <div
      className="fd-freshness-bar"
      data-stale-level={level}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: space[2],
        padding: `${space[1]}px ${space[2]}px ${space[1]}px ${space[3]}px`,
        ...glassSurfaceStyle(theme, { borderRadius: radius.pill, material: "ultraThin" }),
        fontSize: fontSize.md,
        color: textColor,
        borderColor: borderColor ?? undefined,
        boxShadow: borderColor
          ? `inset 0 0 0 1px ${borderColor}88`
          : undefined,
      }}
    >
      <Icon size={14} aria-hidden color={borderColor || undefined} />
      <span className="fd-tabular-nums">{label}</span>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        disabled={isFetching || offline}
        onClick={onRefresh}
        aria-label={t("overview.refreshData")}
        aria-busy={isFetching}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: space[1],
          fontWeight: fontWeight.medium,
          minHeight: 28,
        }}
      >
        <ArrowsClockwise size={14} className={isFetching ? "fd-spin" : undefined} aria-hidden />
        {isFetching ? t("overview.refreshing") : t("overview.refreshData")}
      </Button>
    </div>
  );
}
