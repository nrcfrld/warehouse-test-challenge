import { useRef } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowDownToLine, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { useLocations, useReceiveStock } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
import { ErrorState } from "./states";
const schema = z.object({
  location_id: z.string().uuid("Choose a location"),
  qty: z
    .string()
    .regex(
      /^[0-9]{1,15}(\.[0-9]{1,3})?$/,
      "Use up to 15 integer and 3 decimal digits",
    )
    .refine((v) => /[1-9]/.test(v), "Quantity must be greater than zero"),
});
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
  const attempt = useRef<{ signature: string; key: string } | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { location_id: "", qty: "" },
  });
  async function submit(values: FormValues) {
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
          if (field.field === "qty" || field.field === "location_id")
            setError(field.field, { message: field.reason });
        if (error.status === 0 || error.status >= 500) {
          toast.error(
            "Receipt outcome is unknown. Retry with the same values to safely check the result.",
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
      title="Receive stock"
      description="Choose a location and enter the quantity arriving at your warehouse."
    >
      {locations.isError ? (
        <ErrorState
          error={locations.error}
          retry={() => void locations.refetch()}
        />
      ) : (
        <form
          onSubmit={handleSubmit(submit)}
          noValidate
          className="mt-7 space-y-5"
        >
          <div>
            <label htmlFor="location">Storage location</label>
            <select
              id="location"
              {...register("location_id")}
              disabled={locations.isPending || mutation.isPending}
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
              disabled={mutation.isPending}
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
                locations.isPending ||
                !locations.data?.data.length ||
                mutation.isPending
              }
            >
              {mutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ArrowDownToLine />
              )}
              Receive stock
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
