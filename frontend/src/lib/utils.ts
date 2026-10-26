import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
export function quantity(value: string) {
  const [integer, fraction = ""] = value.split(".");
  const trimmed = fraction.replace(/0+$/, "");
  return (
    integer.replace(/\B(?=(\d{3})+(?!\d))/g, ",") +
    (trimmed ? `.${trimmed}` : "")
  );
}
