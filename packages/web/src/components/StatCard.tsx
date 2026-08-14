import { memo, useEffect, useRef, useState } from "react";
import { Text } from "@cloudflare/kumo";
import { useAppStore } from "../stores/appStore";
import { getTheme, space, fontSize, fontWeight, letterSpacing, radius } from "../styles/theme";
import { Card } from "./ui/Card";

// 红涨绿跌（国内惯例）— colors from theme tokens only
interface Props {
  label: string;
  value: string;
  color?: "up" | "down";
  sub?: string;
  glass?: boolean;
  /** Soft accent bar on the left (hero cards). */
  accent?: "up" | "down" | "neutral";
  /** Emphasize primary KPI (larger value). */
  emphasis?: boolean;
}

export default memo(function StatCard({
  label,
  value,
  color,
  sub,
  glass = true,
  accent,
  emphasis = false,
}: Props) {
  const dark = useAppStore((s) => s.dark);
  const t = getTheme(dark);
  const valueColor = color === "up" ? t.up : color === "down" ? t.down : undefined;
  const [flash, setFlash] = useState<"up" | "down" | null>(null);
  const prev = useRef(value);

  useEffect(() => {
    if (prev.current === value) return;
    // Only flash when color encodes direction (PnL / XIRR style).
    if (color === "up" || color === "down") {
      setFlash(color);
      const id = window.setTimeout(() => setFlash(null), 300);
      prev.current = value;
      return () => window.clearTimeout(id);
    }
    prev.current = value;
  }, [value, color]);

  const accentColor =
    accent === "up" ? t.up : accent === "down" ? t.down : accent === "neutral" ? t.blue : undefined;

  return (
    <Card dark={dark} glass={glass} padded={false}>
      <div
        className={`fd-stat-card${flash ? ` fd-stat-flash-${flash}` : ""}`}
        style={{
          padding: `${space[4]}px ${space[5]}px`,
          position: "relative",
          overflow: "hidden",
          borderRadius: radius.xl,
          transition: "transform var(--fd-duration-fast) var(--fd-easing-standard), box-shadow var(--fd-duration-normal) var(--fd-easing-standard)",
        }}
      >
        {accentColor && (
          <span
            aria-hidden
            style={{
              position: "absolute",
              left: 0,
              top: space[3],
              bottom: space[3],
              width: 3,
              borderRadius: 2,
              background: accentColor,
              opacity: 0.9,
            }}
          />
        )}
        <Text variant="secondary" as="span" size="xs">
          {label}
        </Text>
        <div style={{ marginTop: space[2] }}>
          <Text
            variant="heading2"
            as="span"
            className="fd-tabular-nums"
            style={{
              ...(valueColor
                ? { color: valueColor, fontWeight: fontWeight.bold, fontSize: emphasis ? fontSize["4xl"] : fontSize["3xl"] }
                : { fontWeight: fontWeight.bold, fontSize: emphasis ? fontSize["4xl"] : fontSize["3xl"] }),
              fontVariantNumeric: "tabular-nums",
              letterSpacing: letterSpacing.tight,
              transition: "color var(--fd-duration-normal) var(--fd-easing-standard)",
            }}
          >
            {value}
          </Text>
        </div>
        {sub && (
          <div style={{ marginTop: space[1] }}>
            <Text variant="secondary" as="span" size="xs" className="fd-tabular-nums">
              {sub}
            </Text>
          </div>
        )}
      </div>
    </Card>
  );
});
