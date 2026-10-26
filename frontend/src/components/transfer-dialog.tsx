import { useRef } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowRightLeft, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { useLocations, useTransferStock } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import type { StockSummary } from "@/types";
import { quantity } from "@/lib/utils";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
import { ErrorState } from "./states";
const schema = z
  .object({
    from_location_id: z.string().uuid("Choose a source location"),
    to_location_id: z.string().uuid("Choose a destination location"),
    qty: z
      .string()
      .regex(
        /^[0-9]{1,15}(\.[0-9]{1,3})?$/,
        "Use up to 15 integer and 3 decimal digits",
      )
      .refine((v) => /[1-9]/.test(v), "Quantity must be greater than zero"),
  })
  .refine((v) => v.from_location_id !== v.to_location_id, {
    path: ["to_location_id"],
    message: "Choose a different destination location",
  });
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
  const attempt = useRef<{ signature: string; key: string } | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    setError,
    watch,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { from_location_id: "", to_location_id: "", qty: "" },
  });
  const source = watch("from_location_id");
  const sourceStock = stock?.locations.find((s) => s.location.id === source);
  async function submit(values: Values) {
    const input = { item_id: itemID, ...values };
    const signature = JSON.stringify(input);
    if (attempt.current?.signature !== signature)
      attempt.current = { signature, key: crypto.randomUUID() };
    try {
      const result = await mutation.mutateAsync({
        input,
        key: attempt.current.key,
      });
      toast.success(result.message);
      attempt.current = null;
      reset();
      onOpenChange(false);
    } catch (error) {
      if (error instanceof APIError) {
        for (const field of error.fields)
          if (
            field.field === "qty" ||
            field.field === "from_location_id" ||
            field.field === "to_location_id"
          )
            setError(field.field, { message: field.reason });
        if (error.status === 0 || error.status >= 500) {
          toast.error(
            "Transfer outcome is unknown. Retry with the same values to safely check the result.",
          );
          return;
        }
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
      {locations.isError ? (
        <ErrorState
          error={locations.error}
          retry={() => void locations.refetch()}
        />
      ) : (
        <form
          noValidate
          onSubmit={handleSubmit(submit)}
          className="mt-7 space-y-5"
        >
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
                disabled={locations.isPending || mutation.isPending}
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
              disabled={mutation.isPending}
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
                locations.isPending ||
                (locations.data?.data.length ?? 0) < 2 ||
                mutation.isPending
              }
            >
              {mutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ArrowRightLeft />
              )}
              Transfer stock
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
