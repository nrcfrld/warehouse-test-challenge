import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { z } from "zod";
import { useStockAttempt } from "@/hooks/use-stock-attempt";
import { transferFormSchema as schema } from "@/lib/stock-validation";
import { ArrowRightLeft, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { useLocations, useTransferStock } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import type { StockSummary } from "@/types";
import { quantity } from "@/lib/utils";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
import { ErrorState } from "./states";
type Values = z.infer<typeof schema>;
export function TransferDialog({
  open,
  onOpenChange,
  itemID,
  unit,
  stock,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  itemID: string;
  unit: string;
  stock?: StockSummary;
}) {
  const locations = useLocations(open);
  const mutation = useTransferStock();
  const saved = useStockAttempt("transfer", itemID);
  const pending = saved.attempt;
  const {
    register,
    handleSubmit,
    reset,
    setError,
    watch,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: pending?.input ?? {
      from_location_id: "",
      to_location_id: "",
      qty: "",
    },
  });
  const source = watch("from_location_id");
  const sourceStock = stock?.locations.find((s) => s.location.id === source);
  async function submit(values: Values) {
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
      reset({ from_location_id: "", to_location_id: "", qty: "" });
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
          if (
            field.field === "qty" ||
            field.field === "from_location_id" ||
            field.field === "to_location_id"
          )
            setError(field.field, { message: field.reason });
      } else if (attempt) {
        toast.error(
          "Transfer outcome is unknown. Use the saved request to safely check the result.",
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
      title="Transfer stock"
      description="Move stock between locations. The total quantity stays the same."
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
          noValidate
          onSubmit={
            pending
              ? (event) => {
                  event.preventDefault();
                  void submit(pending.input);
                }
              : handleSubmit(submit)
          }
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

          {(
            [
              { name: "from_location_id", label: "From location" },
              { name: "to_location_id", label: "To location" },
            ] as const
          ).map((field) => (
            <div key={field.name}>
              <label htmlFor={field.name}>{field.label}</label>
              <select
                id={field.name}
                {...register(field.name)}
                disabled={
                  Boolean(pending) || locations.isPending || mutation.isPending
                }
                aria-invalid={Boolean(errors[field.name])}
                aria-describedby={
                  errors[field.name] ? `${field.name}-error` : undefined
                }
              >
                <option value="">
                  {locations.isPending
                    ? "Loading locations…"
                    : "Choose a location"}
                </option>
                {locations.data?.data
                  .filter(
                    (l) => field.name === "from_location_id" || l.id !== source,
                  )
                  .map((location) => (
                    <option key={location.id} value={location.id}>
                      {location.code} · {location.zone}
                    </option>
                  ))}
              </select>
              {errors[field.name] && (
                <p className="field-error" id={`${field.name}-error`}>
                  {errors[field.name]?.message}
                </p>
              )}
              {field.name === "from_location_id" && source && (
                <p
                  className="mt-2 text-xs text-muted-foreground"
                  aria-live="polite"
                >
                  Available: {quantity(sourceStock?.qty ?? "0.000")} {unit}
                </p>
              )}
            </div>
          ))}
          <div>
            <label htmlFor="transfer-qty">
              Quantity{" "}
              <span className="font-normal text-muted-foreground">
                ({unit})
              </span>
            </label>
            <input
              id="transfer-qty"
              inputMode="decimal"
              placeholder="0.000"
              {...register("qty")}
              disabled={Boolean(pending) || mutation.isPending}
              aria-invalid={Boolean(errors.qty)}
              aria-describedby={
                errors.qty ? "transfer-qty-error" : "transfer-qty-help"
              }
            />
            {errors.qty ? (
              <p className="field-error" id="transfer-qty-error">
                {errors.qty.message}
              </p>
            ) : (
              <p
                id="transfer-qty-help"
                className="mt-2 text-xs text-muted-foreground"
              >
                Up to 3 decimal places. Must not exceed the source balance.
              </p>
            )}
          </div>
          {locations.data && locations.data.data.length < 2 && (
            <p role="alert" className="text-sm text-amber-800">
              Add at least two locations before transferring stock.
            </p>
          )}
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
                  (locations.isPending ||
                    (locations.data?.data.length ?? 0) < 2))
              }
            >
              {mutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ArrowRightLeft />
              )}
              {pending ? "Check saved transfer" : "Transfer stock"}
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
