import { useRef } from "react";
// Controlled dialogs have no Radix Trigger; restore focus explicitly.
export function useDialogFocus() {
  const opener = useRef<HTMLElement | null>(null);
  return {
    onOpenAutoFocus: () => {
      opener.current =
        document.activeElement instanceof HTMLElement
          ? document.activeElement
          : null;
    },
    onCloseAutoFocus: (event: Event) => {
      event.preventDefault();
      const target = opener.current?.isConnected
        ? opener.current
        : document.getElementById("main");
      target?.focus();
    },
  };
}
