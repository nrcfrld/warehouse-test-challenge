import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  Package,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useCategories, useItems } from "@/hooks/use-warehouse";
import { SearchField } from "@/components/search-field";
import { CategoryFilter } from "@/components/category-filter";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/icon-button";
import { DeleteDialog } from "@/components/delete-dialog";
import { EmptyState, ErrorState, Loading } from "@/components/states";
import type { Item } from "@/types";
function SearchInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const [input, setInput] = useState(value);
  useEffect(() => {
    setInput(value);
  }, [value]);
  useEffect(() => {
    if (input === value) return;
    const timer = window.setTimeout(() => onChange(input.trim()), 350);
    return () => clearTimeout(timer);
  }, [input, value, onChange]);
  return (
    <SearchField
      label="Search items"
      placeholder="Search by name or SKU…"
      value={input}
      onValueChange={setInput}
      maxLength={200}
    />
  );
}
export default function ItemsPage() {
  const [params, setParams] = useSearchParams();
  const pageValue = Number(params.get("page") || 1);
  const page =
    Number.isInteger(pageValue) && pageValue > 0 && pageValue <= 1000000
      ? pageValue
      : 1;
  const filters = {
    search: params.get("search") || "",
    category: params.get("category") || "",
    page,
    limit: 10,
  };
  const query = useItems(filters);
  const categories = useCategories();
  const [deleting, setDeleting] = useState<Item | null>(null);
  const total = query.data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / filters.limit));
  const filtered = Boolean(filters.search || filters.category);
  const change = useCallback(
    (key: string, value: string) => {
      setParams(
        (current) => {
          const next = new URLSearchParams(current);
          if (value) next.set(key, value);
          else next.delete(key);
          if (key !== "page") next.delete("page");
          return next;
        },
        { replace: key === "search" },
      );
    },
    [setParams],
  );
  const changeSearch = useCallback(
    (value: string) => change("search", value),
    [change],
  );
  useEffect(() => {
    if (query.data && !query.isPlaceholderData && page > pages)
      setParams(
        (current) => {
          const next = new URLSearchParams(current);
          next.set("page", String(pages));
          return next;
        },
        { replace: true },
      );
  }, [query.data, query.isPlaceholderData, page, pages, setParams]);
  return (
    <div className="page">
      <div className="mb-9 flex items-end justify-between gap-4">
        <div>
          <p className="eyebrow mb-3">Your warehouse, organized</p>
          <h1 className="text-[34px] font-semibold leading-tight tracking-[-0.045em]">
            Inventory
            <span className="ml-3 inline-flex translate-y-[-4px] rounded-lg border bg-white px-2.5 py-1 align-middle text-xs font-medium tracking-normal text-muted-foreground">
              {query.data ? total : "—"} items
            </span>
          </h1>
          <p className="mt-3 text-sm text-muted-foreground">
            A clear view of what you have, and where it belongs.
          </p>
        </div>
        <IconButton
          label="Add item"
          variant="default"
          className="size-12 rounded-2xl"
          asChild
        >
          <Link to="/items/new">
            <Plus className="size-5!" />
          </Link>
        </IconButton>
      </div>
      <section className="panel" aria-label="Inventory items">
        <div className="flex flex-wrap items-center justify-between gap-4 border-b p-5">
          <SearchInput value={filters.search} onChange={changeSearch} />
          <div className="flex items-center gap-3">
            <CategoryFilter
              value={filters.category}
              categories={categories.data?.data ?? []}
              onValueChange={(value) => change("category", value)}
              loading={categories.isPending}
            />
            <IconButton
              label="Refresh items"
              disabled={query.isFetching}
              onClick={() => {
                void query.refetch();
                void categories.refetch();
              }}
            >
              <RefreshCw className={query.isFetching ? "animate-spin" : ""} />
            </IconButton>
          </div>
        </div>
        {categories.isError && (
          <div
            role="alert"
            className="flex items-center justify-between border-b bg-amber-50 px-5 py-2 text-sm text-amber-900"
          >
            Categories could not be loaded.
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void categories.refetch()}
            >
              Retry
            </Button>
          </div>
        )}
        {query.isPending ? (
          <Loading />
        ) : query.isError ? (
          <ErrorState error={query.error} retry={() => void query.refetch()} />
        ) : query.data.data.length === 0 ? (
          <EmptyState filtered={filtered}>
            {filtered ? (
              <Button
                variant="outline"
                className="mt-6"
                onClick={() => setParams({})}
              >
                Clear filters
              </Button>
            ) : (
              <Button asChild className="mt-6">
                <Link to="/items/new">
                  <Plus />
                  Add your first item
                </Link>
              </Button>
            )}
          </EmptyState>
        ) : (
          <div
            className="relative overflow-x-auto"
            aria-busy={query.isFetching}
          >
            <table className="w-full border-collapse text-left">
              <thead className="bg-[#fbfcfd] text-[11px] uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="px-6 py-4 font-medium">Item</th>
                  <th className="px-5 py-4 font-medium">SKU</th>
                  <th className="px-5 py-4 font-medium">Category</th>
                  <th className="px-5 py-4 font-medium">Unit</th>
                  <th className="px-4 py-4 text-right font-medium">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className={query.isPlaceholderData ? "opacity-60" : ""}>
                {query.data.data.map((item) => (
                  <tr
                    key={item.id}
                    className="group border-t transition-colors hover:bg-[#fafbfc]"
                  >
                    <td className="min-w-60 px-6 py-5">
                      <div className="flex items-center gap-3.5">
                        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl border bg-[#f8f9fb] text-[#7b8594]">
                          <Package className="size-5" aria-hidden="true" />
                        </div>
                        <Link
                          to={`/items/${item.id}`}
                          className="text-sm font-medium leading-5 hover:text-primary"
                        >
                          {item.name}
                        </Link>
                      </div>
                    </td>
                    <td className="whitespace-nowrap px-5 py-5 font-mono text-xs text-muted-foreground">
                      {item.sku}
                    </td>
                    <td className="px-5 py-5">
                      <span className="inline-block whitespace-nowrap rounded-md bg-muted px-2.5 py-1 text-xs text-muted-foreground">
                        {item.category}
                      </span>
                    </td>
                    <td className="px-5 py-5 text-sm text-muted-foreground">
                      {item.unit}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end">
                        <IconButton label={`View ${item.name}`} asChild>
                          <Link to={`/items/${item.id}`}>
                            <ArrowUpRight />
                          </Link>
                        </IconButton>
                        <IconButton label={`Edit ${item.name}`} asChild>
                          <Link to={`/items/${item.id}/edit`}>
                            <Pencil />
                          </Link>
                        </IconButton>
                        <IconButton
                          label={`Delete ${item.name}`}
                          onClick={() => setDeleting(item)}
                          className="hover:bg-red-50 hover:text-red-600"
                        >
                          <Trash2 />
                        </IconButton>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {query.data && total > 0 && (
          <div className="flex items-center justify-between border-t px-6 py-3">
            <p className="text-xs text-muted-foreground">
              Showing{" "}
              <span className="font-medium text-foreground">
                {Math.min((page - 1) * filters.limit + 1, total)}–
                {Math.min(page * filters.limit, total)}
              </span>{" "}
              of {total} items
            </p>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">
                Page {page} of {pages}
              </span>
              <IconButton
                label="Previous page"
                disabled={page <= 1 || query.isPlaceholderData}
                onClick={() => change("page", String(page - 1))}
              >
                <ChevronLeft />
              </IconButton>
              <IconButton
                label="Next page"
                disabled={page >= pages || query.isPlaceholderData}
                onClick={() => change("page", String(page + 1))}
              >
                <ChevronRight />
              </IconButton>
            </div>
          </div>
        )}
      </section>
      <p className="mt-5 flex items-center gap-2 text-xs text-muted-foreground">
        <Package className="size-3.5" aria-hidden="true" />
        Select an item to view its locations and receive stock.
      </p>
      <DeleteDialog item={deleting} onClose={() => setDeleting(null)} />
    </div>
  );
}
