export const PAGE_SIZE = 100;

export const DIRECTIONS = [
  { value: "", label: "全部方向" },
  { value: "buy", label: "买入" },
  { value: "sell", label: "卖出" },
  { value: "dividend", label: "分红" },
];

export const DIRECTION_BADGE: Record<string, "up" | "down" | "accent" | "neutral"> = {
  buy: "up",
  sell: "down",
  dividend: "accent",
};
