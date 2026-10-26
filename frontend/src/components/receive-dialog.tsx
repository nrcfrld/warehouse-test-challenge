import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { z } from "zod";
import { useStockAttempt } from "@/hooks/use-stock-attempt";
import { receiveFormSchema as schema } from "@/lib/stock-validation";
import { ArrowDownToLine, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { useLocations, useReceiveStock } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
import { ErrorState } from "./states";
type FormValues = z.infer<typeof schema>;
export function ReceiveDialog({
  open,
  onOpenChange,
  itemID,
  unit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  itemID: string;
  unit: string;
}) {
  const locations = useLocations(open);
  const mutation = useReceiveStock();
  const saved = useStockAttempt("receive", itemID);
  const pending = saved.attempt;
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: pending?.input ?? { location_id: "", qty: "" },
  });
  async function submit(values: FormValues) {
    let attempt: typeof pending = null;
    try {
      attempt = saved.prepare({ item_id: itemID, ...values });
      const result = await mutation.mutateAsync({
        input: attempt.input,
        key: attempt.key,
      });
      const storageError = saved.resolve(attempt.key);
      if (storageError) {
        toast.error(storageError.message);
        return;
      }
      toast.success(result.message);
      reset({ location_id: "", qty: "" });
      onOpenChange(false);
    } catch (error) {
      if (
        error instanceof APIError &&
        error.status >= 400 &&
        error.status < 500 &&
        error.status !== 408
      ) {
        if (attempt) {
          const storageError = saved.resolve(attempt.key);
          if (storageError) {
            toast.error(storageError.message);
            return;
          }
        }
        for (const field of error.fields)
          if (field.field === "qty" || field.field === "location_id")
            setError(field.field, { message: field.reason });
      } else if (attempt) {
        toast.error(
          "Receipt outcome is unknown. Use the saved request to safely check the result.",
        );
        return;
      }
      toast.error((error as Error).message);
    }
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!mutation.isPending) onOpenChange(next);
      }}
      title="Receive stock"
      description="Choose a location and enter the quantity arriving at your warehouse."
    >
      {saved.error ? (
        <ErrorState error={saved.error} />
      ) : locations.isError && !pending ? (
        <ErrorState
          error={locations.error}
          retry={() => void locations.refetch()}
        />
      ) : (
        <form
          onSubmit={
            pending
              ? (event) => {
                  event.preventDefault();
                  void submit(pending.input);
                }
              : handleSubmit(submit)
          }
          noValidate
          className="mt-7 space-y-5"
        >
          {pending && (
            <p
              role="status"
              className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-950"
            >
              This request is saved. Checking its result uses the original
              values and will not apply it twice. You can return to this item
              after navigating away or reloading this tab.
            </p>
          )}

          <div>
            <label htmlFor="location">Storage location</label>
            <select
              id="location"
              {...register("location_id")}
              disabled={
                Boolean(pending) || locations.isPending || mutation.isPending
              }
              aria-invalid={Boolean(errors.location_id)}
              aria-describedby={
                errors.location_id ? "location-error" : undefined
              }
            >
              <option value="">
                {locations.isPending
                  ? "Loading locations…"
                  : "Choose a location"}
              </option>
              {locations.data?.data.map((location) => (
                <option key={location.id} value={location.id}>
                  {location.code} · {location.zone}
                </option>
              ))}
            </select>
            {errors.location_id && (
              <p className="field-error" id="location-error">
                {errors.location_id.message}
              </p>
            )}
            {locations.data?.data.length === 0 && (
              <p role="alert" className="mt-2 text-sm text-amber-800">
                No locations are available. Add a location from the Locations
                menu before receiving stock.
              </p>
            )}
          </div>
          <div>
            <label htmlFor="qty">
              Quantity{" "}
              <span className="font-normal text-muted-foreground">
                ({unit})
              </span>
            </label>
            <input
              id="qty"
              inputMode="decimal"
              placeholder="0.000"
              {...register("qty")}
              disabled={Boolean(pending) || mutation.isPending}
              aria-invalid={Boolean(errors.qty)}
              aria-describedby={errors.qty ? "qty-error" : "qty-help"}
            />
            {errors.qty ? (
              <p className="field-error" id="qty-error">
                {errors.qty.message}
              </p>
            ) : (
              <p id="qty-help" className="mt-2 text-xs text-muted-foreground">
                Up to 3 decimal places. Use a period as the decimal separator.
              </p>
            )}
          </div>
          <div className="flex justify-end gap-3 pt-3">
            <Button
              variant="outline"
              type="button"
              disabled={mutation.isPending}
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={
                mutation.isPending ||
                (!pending &&
                  (locations.isPending || !locations.data?.data.length))
              }
            >
              {mutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ArrowDownToLine />
              )}
              {pending ? "Check saved receipt" : "Receive stock"}
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
