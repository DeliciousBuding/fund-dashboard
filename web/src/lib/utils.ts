// cn — clsx + tailwind-merge（shadcn 惯例，类名合并唯一出处）
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
