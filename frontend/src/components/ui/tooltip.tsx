import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
export const TooltipProvider = TooltipPrimitive.Provider;
export function Tooltip({
  children,
  content,
}: {
  children: ReactNode;
  content: string;
}) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          sideOffset={7}
          className="z-50 rounded-lg bg-zinc-900 px-3 py-2 text-xs font-medium text-white shadow-lg"
        >
          {content}
          <TooltipPrimitive.Arrow className="fill-zinc-900" />
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
