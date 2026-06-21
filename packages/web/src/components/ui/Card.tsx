// Card — unified surface container for v3.0 dashboard.
// Single source of card styling (radius/border/shadow/padding) replacing the
// mix of LayerCard + raw divs across components. Theme tokens via getTheme(dark).
import { useState, type ReactNode, type CSSProperties, type MouseEvent } from "react";
import { getTheme } from "../../styles/theme";

interface CardProps {
  dark: boolean;
  children: ReactNode;
  style?: CSSProperties;
  hover?: boolean;
  onClick?: (e: MouseEvent<HTMLDivElement>) => void;
  padded?: boolean;
}

export function Card({ dark, children, style, hover, onClick, padded = true }: CardProps) {
  const t = getTheme(dark);
  const [h, setH] = useState(false);
  const activeHover = !!hover && h;
  return (
    <div
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      style={{
        background: t.surface,
        border: `1px solid ${activeHover ? t.blue : t.border}`,
        borderRadius: 12,
        boxShadow: activeHover ? t.shadowHover : t.shadowCard,
        padding: padded ? 20 : 0,
        transition: "box-shadow .2s ease, border-color .2s ease, transform .2s ease",
        transform: activeHover ? "translateY(-1px)" : "none",
        ...(hover ? { cursor: "pointer" } : {}),
        ...style,
      }}
    >
      {children}
    </div>
  );
}
