import { Link, useNavigate, useParams } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Check, LoaderCircle, Package } from "lucide-react";
import { toast } from "sonner";
import { useItem, useItemMutation } from "@/hooks/use-warehouse";
import { APIError } from "@/services/api";
import type { Item, ItemInput } from "@/types";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/icon-button";
import { ErrorState, Loading } from "@/components/states";
const schema = z.object({
  sku: z
    .string()
    .trim()
    .min(1, "SKU is required")
    .max(64)
    .transform((v) => v.toUpperCase()),
  name: z.string().trim().min(1, "Name is required").max(200),
  category: z.string().trim().min(1, "Category is required").max(100),
  unit: z.string().trim().min(1, "Unit is required").max(32),
});
function ItemForm({ item }: { item?: Item }) {
  const navigate = useNavigate();
  const mutation = useItemMutation(item?.id);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<ItemInput>({
    resolver: zodResolver(schema),
    defaultValues: item ?? { sku: "", name: "", category: "", unit: "" },
  });
  async function submit(input: ItemInput) {
    try {
      const result = await mutation.mutateAsync(input);
      toast.success(result.message);
      navigate(`/items/${result.data.id}`);
    } catch (error) {
      if (error instanceof APIError)
        for (const field of error.fields)
          if (field.field in schema.shape)
            setError(field.field as keyof ItemInput, { message: field.reason });
      toast.error((error as Error).message);
    }
  }
  return (
    <form
      onSubmit={handleSubmit(submit)}
      noValidate
      className="panel max-w-3xl"
    >
      <div className="border-b px-8 py-6">
        <h2 className="font-semibold">Item information</h2>
        <p className="mt-1.5 text-sm text-muted-foreground">
          The essentials that keep your inventory consistent.
        </p>
      </div>
      <div className="space-y-6 p-8">
        <div>
          <label htmlFor="name">Item name</label>
          <input
            id="name"
            placeholder="e.g. Recycled shipping box"
            maxLength={200}
            autoFocus
            {...register("name")}
            aria-invalid={Boolean(errors.name)}
            aria-describedby={errors.name ? "name-error" : undefined}
            disabled={mutation.isPending}
          />
          {errors.name && (
            <p id="name-error" className="field-error">
              {errors.name.message}
            </p>
          )}
        </div>
        <div className="grid gap-6 sm:grid-cols-2">
          <div>
            <label htmlFor="sku">SKU</label>
            <input
              id="sku"
              placeholder="e.g. PKG-001"
              maxLength={64}
              className="font-mono uppercase"
              {...register("sku")}
              aria-invalid={Boolean(errors.sku)}
              aria-describedby={errors.sku ? "sku-error" : "sku-help"}
              disabled={mutation.isPending}
            />
            {errors.sku ? (
              <p id="sku-error" className="field-error">
                {errors.sku.message}
              </p>
            ) : (
              <p id="sku-help" className="mt-2 text-xs text-muted-foreground">
                A unique identifier, saved in uppercase.
              </p>
            )}
          </div>
          <div>
            <label htmlFor="category">Category</label>
            <input
              id="category"
              placeholder="e.g. Packaging"
              maxLength={100}
              {...register("category")}
              aria-invalid={Boolean(errors.category)}
              aria-describedby={errors.category ? "category-error" : undefined}
              disabled={mutation.isPending}
            />
            {errors.category && (
              <p id="category-error" className="field-error">
                {errors.category.message}
              </p>
            )}
          </div>
        </div>
        <div className="sm:max-w-[calc(50%-0.75rem)]">
          <label htmlFor="unit">Unit of measure</label>
          <input
            id="unit"
            placeholder="e.g. pcs, kg, m"
            maxLength={32}
            {...register("unit")}
            aria-invalid={Boolean(errors.unit)}
            aria-describedby={errors.unit ? "unit-error" : "unit-help"}
            disabled={mutation.isPending}
          />
          {errors.unit ? (
            <p id="unit-error" className="field-error">
              {errors.unit.message}
            </p>
          ) : (
            <p
              id="unit-help"
              className="mt-2 text-xs leading-5 text-muted-foreground"
            >
              All stock quantities use this unit.
              {item && " Changing it does not convert existing quantities."}
            </p>
          )}
        </div>
      </div>
      <div className="flex items-center justify-between border-t bg-[#fcfcfd] px-8 py-5">
        <span className="text-xs text-muted-foreground">
          All fields are required
        </span>
        <div className="flex gap-3">
          <Button
            variant="outline"
            type="button"
            onClick={() => navigate(item ? `/items/${item.id}` : "/items")}
            disabled={mutation.isPending}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? (
              <LoaderCircle className="animate-spin" />
            ) : (
              <Check />
            )}
            {item ? "Save changes" : "Create item"}
          </Button>
        </div>
      </div>
    </form>
  );
}
export default function ItemFormPage() {
  const { id = "" } = useParams();
  const query = useItem(id);
  return (
    <div className="page">
      <div className="mb-8">
        <IconButton label="Back to inventory" asChild className="-ml-3 mb-4">
          <Link to={id ? `/items/${id}` : "/items"}>
            <ArrowLeft />
          </Link>
        </IconButton>
        <p className="eyebrow mb-3">Item catalog</p>
        <h1 className="text-[32px] font-semibold tracking-tight">
          {id ? "Edit item" : "New item"}
        </h1>
        <p className="mt-3 text-sm text-muted-foreground">
          {id
            ? "Keep your item details up to date."
            : "A little detail now. A more organized warehouse ahead."}
        </p>
      </div>
      {id && query.isPending ? (
        <div className="panel max-w-3xl">
          <Loading rows={4} />
        </div>
      ) : id && query.isError ? (
        <ErrorState error={query.error} retry={() => void query.refetch()} />
      ) : (
        <ItemForm key={id} item={query.data?.data} />
      )}
      <p className="mt-5 flex items-center gap-2 text-xs text-muted-foreground">
        <Package className="size-3.5" aria-hidden="true" />
        Stock is managed separately from item details.
      </p>
    </div>
  );
}
