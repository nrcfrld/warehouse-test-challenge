import { useDialogFocus } from "@/hooks/use-dialog-focus";
import * as Primitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "./button";
import { Tooltip } from "./tooltip";
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  children: ReactNode;
}) {
  const focus = useDialogFocus();
  return (
    <Primitive.Root open={open} onOpenChange={onOpenChange}>
      <Primitive.Portal>
        <Primitive.Overlay className="fixed inset-0 z-40 bg-black/25 backdrop-blur-[3px]" />
        <Primitive.Content
          {...focus}
          className="fixed left-1/2 top-1/2 z-50 max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-3xl border border-border bg-white p-8 shadow-2xl"
        >
          <div className="pr-8">
            <Primitive.Title className="text-xl font-semibold tracking-tight">
              {title}
            </Primitive.Title>
            <Primitive.Description className="mt-2 text-sm leading-6 text-muted-foreground">
              {description}
            </Primitive.Description>
          </div>
          <Tooltip content="Close">
            <Primitive.Close asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Close dialog"
                className="absolute right-4 top-4"
              >
                <X aria-hidden="true" />
              </Button>
            </Primitive.Close>
          </Tooltip>
          {children}
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  );
}
