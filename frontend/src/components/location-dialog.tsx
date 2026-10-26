import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { LoaderCircle, MapPin } from "lucide-react";
import { toast } from "sonner";
import { useLocationMutation } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import type { Location } from "@/types";
import { Dialog } from "./ui/dialog";
import { Button } from "./ui/button";
const schema = z.object({
  code: z.string().trim().min(1, "Location code is required").max(32),
  zone: z.string().trim().min(1, "Zone is required").max(100),
  type: z.string().trim().min(1, "Type is required").max(50),
});
type Values = z.infer<typeof schema>;
export function LocationDialog({
  location,
  onClose,
}: {
  location?: Location;
  onClose: () => void;
}) {
  const mutation = useLocationMutation(location?.id);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      code: location?.code ?? "",
      zone: location?.zone ?? "",
      type: location?.type ?? "",
    },
  });
  async function submit(values: Values) {
    try {
      const result = await mutation.mutateAsync(values);
      toast.success(result.message);
      onClose();
    } catch (error) {
      if (error instanceof APIError)
        for (const field of error.fields)
          if (
            field.field === "code" ||
            field.field === "zone" ||
            field.field === "type"
          )
            setError(field.field, { message: field.reason });
      toast.error((error as Error).message);
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
      title={location ? "Edit location" : "Add location"}
      description="Give this storage position a unique code, zone, and type."
    >
      <form
        onSubmit={handleSubmit(submit)}
        noValidate
        className="mt-7 space-y-5"
      >
        {(
          [
            {
              name: "code",
              label: "Location code",
              placeholder: "C-01",
              max: 32,
            },
            {
              name: "zone",
              label: "Zone",
              placeholder: "Main warehouse",
              max: 100,
            },
            { name: "type", label: "Type", placeholder: "Shelf", max: 50 },
          ] as const
        ).map((field) => (
          <div key={field.name}>
            <label htmlFor={`location-${field.name}`}>{field.label}</label>
            <input
              id={`location-${field.name}`}
              {...register(field.name)}
              placeholder={field.placeholder}
              maxLength={field.max}
              disabled={mutation.isPending}
              aria-invalid={Boolean(errors[field.name])}
              aria-describedby={
                errors[field.name] ? `${field.name}-error` : undefined
              }
            />
            {errors[field.name] && (
              <p className="field-error" id={`${field.name}-error`}>
                {errors[field.name]?.message}
              </p>
            )}
          </div>
        ))}
        <div className="flex justify-end gap-3 pt-3">
          <Button
            type="button"
            variant="outline"
            disabled={mutation.isPending}
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? (
              <LoaderCircle className="animate-spin" />
            ) : (
              <MapPin />
            )}
            {location ? "Save changes" : "Create location"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
