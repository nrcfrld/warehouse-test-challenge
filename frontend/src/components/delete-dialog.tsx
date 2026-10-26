import { useDialogFocus } from "@/hooks/use-dialog-focus";
import * as AlertDialog from "@radix-ui/react-alert-dialog";
import { Trash2, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import type { Item } from "@/types";
import { useDeleteItem } from "@/hooks/use-warehouse";
import { Button } from "./ui/button";
export function DeleteDialog({
  item,
  onClose,
  onDeleted,
}: {
  item: Item | null;
  onClose: () => void;
  onDeleted?: () => void;
}) {
  const focus = useDialogFocus();
  const mutation = useDeleteItem();
  async function remove() {
    if (!item) return;
    try {
      const result = await mutation.mutateAsync(item.id);
      toast.success(result.message);
      onClose();
      onDeleted?.();
    } catch (error) {
      toast.error((error as Error).message);
    }
  }
  return (
    <AlertDialog.Root
      open={Boolean(item)}
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-40 bg-black/25 backdrop-blur-[3px]" />
        <AlertDialog.Content
          {...focus}
          className="fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-3xl bg-white p-8 shadow-2xl"
        >
          <div className="mb-5 flex size-12 items-center justify-center rounded-2xl bg-red-50 text-red-600">
            <Trash2 aria-hidden="true" />
          </div>
          <AlertDialog.Title className="text-xl font-semibold">
            Delete this item?
          </AlertDialog.Title>
          <AlertDialog.Description className="mt-3 text-sm leading-6 text-muted-foreground">
            <strong className="font-medium text-foreground">
              {item?.name}
            </strong>{" "}
            will be removed from your active inventory. Existing stock and
            transaction history will be preserved. This action cannot be undone
            in the app.
          </AlertDialog.Description>
          <div className="mt-8 flex justify-end gap-3">
            <AlertDialog.Cancel asChild>
              <Button variant="outline" disabled={mutation.isPending}>
                Cancel
              </Button>
            </AlertDialog.Cancel>
            <Button
              variant="destructive"
              onClick={() => void remove()}
              disabled={mutation.isPending}
            >
              {mutation.isPending && <LoaderCircle className="animate-spin" />}
              Delete item
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
