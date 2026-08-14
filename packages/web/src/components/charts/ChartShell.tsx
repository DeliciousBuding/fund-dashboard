// ChartShell — presentational wrapper for every chart card.
// Consolidates the hand-written `placeholder()` tristate (loading/error/empty) +
// Card + header (title/subtitle/range tabs) that was duplicated across 10 charts.
// Purely presentational: the consumer owns useEChart + the chart ref div (passed as children),
// so this component imports NO echarts — keeping the render/data boundary clean.
import { type ReactNode, type CSSProperties } from "react";
import { Text, Tabs } from "@cloudflare/kumo";
import { useTranslation } from "react-i18next";
import { getTheme, space, chartHeight } from "../../styles/theme";
import { Card } from "../ui/Card";

export interface RangeOption {
  value: string;
  label: string;
}

export interface ChartShellProps {
  dark: boolean;
  title?: ReactNode;
  subtitle?: ReactNode;
  /** segmented range tabs (e.g. 1m/3m/1y); omit to hide. */
  ranges?: RangeOption[];
  range?: string;
  onRangeChange?: (v: string) => void;
  /** extra node rendered alongside the range tabs (e.g. a compare Select). */
  headerExtra?: ReactNode;
  /** tristate — first truthy wins; when none, children render. */
  loading?: boolean;
  error?: string | null;
  empty?: boolean;
  loadingText?: string;
  errorText?: string;
  emptyText?: string;
  /** placeholder testid prefix. Default "chart" → chart-loading/chart-error/chart-empty. */
  testidPrefix?: string;
  /** placeholder height (px). Default chartHeight.default — match the consumer's chart div height. */
  height?: number;
  marginBottom?: number;
  style?: CSSProperties;
  children?: ReactNode;
}

export function ChartShell({
  dark,
  title,
  subtitle,
  ranges,
  range,
  onRangeChange,
  headerExtra,
  loading,
  error,
  empty,
  loadingText,
  errorText,
  emptyText,
  testidPrefix = "chart",
  height = chartHeight.default,
  marginBottom,
  style,
  children,
}: ChartShellProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const hasHeader = title != null || subtitle != null || ranges != null || headerExtra != null;

  const placeholder = (msg: string, testid: string) => (
    <div
      data-testid={testid}
      style={{
        height,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: theme.textMuted,
        fontVariantNumeric: "tabular-nums",
      }}
    >
      {msg}
    </div>
  );

  return (
    <Card dark={dark} glass style={{ ...(marginBottom != null ? { marginBottom } : {}), ...style }}>
      {hasHeader && (
        <div
          style={{
            padding: `${space[1]}px 0 ${space[4]}px`,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: space[2],
          }}
        >
          <div>
            {title != null && (
              <Text variant="heading3" as="h3">
                {title}
              </Text>
            )}
            {subtitle != null && (
              <Text variant="secondary" as="span" size="xs" style={{ marginTop: space[1] / 2, display: "block" }}>
                {subtitle}
              </Text>
            )}
          </div>
          {(ranges != null || headerExtra != null) && (
            <div style={{ display: "flex", alignItems: "center", gap: space[2] }}>
              {headerExtra}
              {ranges != null && (
                <Tabs tabs={ranges} value={range} onValueChange={onRangeChange} variant="segmented" size="sm" />
              )}
            </div>
          )}
        </div>
      )}
      {loading
        ? placeholder(loadingText ?? t("common.loading"), `${testidPrefix}-loading`)
        : error
          ? placeholder(errorText ?? error ?? t("common.loadError"), `${testidPrefix}-error`)
          : empty
            ? placeholder(emptyText ?? t("common.noData"), `${testidPrefix}-empty`)
            : children}
    </Card>
  );
}
