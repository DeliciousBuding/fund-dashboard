// Card — unified surface container for v3.0 dashboard.
// Supports solid + multi-tier frosted-glass materials (Apple-like translucent shells).
// Theme tokens via getTheme(dark). Transitions only transform/opacity/shadow/border.
import { useState, type ReactNode, type CSSProperties, type MouseEvent } from "react";
import { getTheme, glassMaterial, radius, space, zIndex, cssTransition, type GlassMaterial } from "../../styles/theme";

interface CardProps {
  dark: boolean;
  children: ReactNode;
  style?: CSSProperties;
  hover?: boolean;
  /** Frosted glass surface (blur + translucent fill). Default false for dense data cards. */
  glass?: boolean;
  /** Glass material tier when glass=true. Default regular. */
  material?: GlassMaterial;
  onClick?: (e: MouseEvent<HTMLDivElement>) => void;
  padded?: boolean;
  className?: string;
  "aria-label"?: string;
}

export function Card({
  dark,
  children,
  style,
  hover,
  glass = false,
  material = "regular",
  onClick,
  padded = true,
  className,
  "aria-label": ariaLabel,
}: CardProps) {
  const t = getTheme(dark);
  const [h, setH] = useState(false);
  const activeHover = !!hover && h;
  const g = glass ? glassMaterial(t, material) : null;

  const base: CSSProperties = g
    ? {
        background: g.surface,
        border: `1px solid ${activeHover ? t.blue : g.border}`,
        // Keep glass identity on hover — intensify glass shadow, do not swap to solid-only.
        boxShadow: activeHover ? `${g.shadow}, ${t.shadowHover}` : g.shadow,
        backdropFilter: g.blur,
        WebkitBackdropFilter: g.blur,
      }
    : {
        background: t.surface,
        border: `1px solid ${activeHover ? t.blue : t.border}`,
        boxShadow: activeHover ? t.shadowHover : t.shadowCard,
      };

  return (
    <div
      className={className ? `fd-card ${className}` : "fd-card"}
      data-glass={glass ? material : undefined}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      aria-label={ariaLabel}
      onClick={onClick}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick(e as unknown as MouseEvent<HTMLDivElement>);
              }
            }
          : undefined
      }
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        position: "relative",
        borderRadius: radius.xl,
        overflow: "hidden",
        padding: padded ? space[5] : 0,
        transition: cssTransition(["box-shadow", "border-color", "transform", "background"]),
        transform: activeHover ? "translateY(-1px)" : "none",
        ...(hover || onClick ? { cursor: "pointer" } : {}),
        ...base,
        ...style,
      }}
    >
      {g && (
        <div
          aria-hidden
          className="fd-card-highlight"
          style={{
            pointerEvents: "none",
            position: "absolute",
            inset: 0,
            background: g.highlight,
            borderRadius: "inherit",
          }}
        />
      )}
      <div style={{ position: "relative", zIndex: zIndex.local }}>{children}</div>
    </div>
  );
}
