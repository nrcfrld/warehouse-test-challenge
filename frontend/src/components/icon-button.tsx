import type { ComponentProps } from "react";
import { Button } from "./ui/button";
import { Tooltip } from "./ui/tooltip";
export function IconButton({
  label,
  children,
  ...props
}: ComponentProps<typeof Button> & { label: string }) {
  return (
    <Tooltip content={label}>
      <Button variant="ghost" size="icon" aria-label={label} {...props}>
        {children}
      </Button>
    </Tooltip>
  );
}
