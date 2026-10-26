import { AlertCircle, PackageOpen, RotateCcw } from "lucide-react";
import { APIError } from "@/services/api";
import { Button } from "./ui/button";
export function Loading({ rows = 5 }: { rows?: number }) {
  return (
    <div role="status" aria-label="Loading data" className="space-y-4 p-8">
      <span className="sr-only">Loading data…</span>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="h-12 animate-pulse rounded-xl bg-muted" />
      ))}
    </div>
  );
}
export function ErrorState({
  error,
  retry,
}: {
  error: Error;
  retry?: () => void;
}) {
  const missing = error instanceof APIError && error.status === 404;
  return (
    <div
      role="alert"
      className="flex flex-col items-center px-8 py-16 text-center"
    >
      <AlertCircle
        className="mb-4 size-7 text-muted-foreground"
        aria-hidden="true"
      />
      <h2 className="font-semibold">
        {missing ? "Item not found" : "Something went wrong"}
      </h2>
      <p className="mt-2 max-w-md text-sm text-muted-foreground">
        {missing
          ? "This item may have been deleted or is no longer available."
          : error.message}
      </p>
      {retry && !missing && (
        <Button variant="outline" onClick={retry} className="mt-6">
          <RotateCcw />
          Try again
        </Button>
      )}
    </div>
  );
}
export function EmptyState({
  filtered = false,
  children,
}: {
  filtered?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center px-8 py-20 text-center">
      <div className="mb-5 flex size-16 items-center justify-center rounded-2xl bg-muted">
        <PackageOpen
          className="size-7 text-muted-foreground"
          aria-hidden="true"
        />
      </div>
      <h2 className="text-lg font-semibold">
        {filtered ? "No matching items" : "A place for every item"}
      </h2>
      <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
        {filtered
          ? "Try a different search or category to find what you need."
          : "Add your first item to start organizing your warehouse and receiving stock."}
      </p>
      {children}
    </div>
  );
}
