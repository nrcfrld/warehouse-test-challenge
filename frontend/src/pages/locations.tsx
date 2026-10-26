import { useState } from "react";
import { MapPin, Pencil, Plus, RefreshCw } from "lucide-react";
import { SearchField } from "@/components/search-field";
import { useLocations } from "@/hooks/use-warehouse";
import { IconButton } from "@/components/icon-button";
import { Button } from "@/components/ui/button";
import { LocationDialog } from "@/components/location-dialog";
import { ErrorState, Loading } from "@/components/states";
import type { Location } from "@/types";
export default function LocationsPage() {
  const query = useLocations();
  const [editor, setEditor] = useState<Location | "new" | null>(null);
  const [search, setSearch] = useState("");
  const locations =
    query.data?.data.filter((location) =>
      [location.code, location.zone, location.type].some((value) =>
        value.toLowerCase().includes(search.trim().toLowerCase()),
      ),
    ) ?? [];
  return (
    <div className="page">
      <div className="mb-9 flex items-end justify-between gap-6">
        <div>
          <p className="eyebrow mb-3">Warehouse workspace</p>
          <h1 className="text-[32px] font-semibold tracking-tight">
            Locations
          </h1>
          <p className="mt-3 text-sm text-muted-foreground">
            A clear home for every item in your warehouse.
          </p>
        </div>
        <Button onClick={() => setEditor("new")}>
          <Plus />
          Add location
        </Button>
      </div>
      <section className="panel" aria-label="Storage locations">
        <div className="flex items-center justify-between gap-4 border-b p-5">
          <SearchField
            label="Search locations"
            placeholder="Search code, zone, or type…"
            value={search}
            onValueChange={setSearch}
          />
          <IconButton
            label="Refresh locations"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw className={query.isFetching ? "animate-spin" : ""} />
          </IconButton>
        </div>
        {query.isPending ? (
          <Loading rows={4} />
        ) : query.isError ? (
          <ErrorState error={query.error} retry={() => void query.refetch()} />
        ) : locations.length === 0 ? (
          <div className="px-8 py-16 text-center">
            <MapPin
              className="mx-auto mb-4 size-8 text-muted-foreground"
              aria-hidden="true"
            />
            <h2 className="font-semibold">
              {search ? "No matching locations" : "No locations yet"}
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {search
                ? "Try a different code, zone, or type."
                : "Add a location to start receiving and moving stock."}
            </p>
            {search && (
              <Button
                className="mt-5"
                variant="outline"
                onClick={() => setSearch("")}
              >
                Clear search
              </Button>
            )}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-[#fbfcfe] text-xs text-muted-foreground">
                <tr>
                  {["Code", "Zone", "Type", "Actions"].map((label) => (
                    <th key={label} className="px-6 py-4 font-medium">
                      {label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {locations.map((location) => (
                  <tr key={location.id} className="border-t">
                    <td className="px-6 py-5 font-medium">
                      <span className="flex items-center gap-2">
                        <MapPin
                          className="size-4 shrink-0 text-muted-foreground"
                          aria-hidden="true"
                        />
                        {location.code}
                      </span>
                    </td>
                    <td className="px-6 py-5 text-muted-foreground">
                      {location.zone}
                    </td>
                    <td className="px-6 py-5 text-muted-foreground">
                      {location.type}
                    </td>
                    <td className="px-6 py-3">
                      <IconButton
                        label={`Edit ${location.code}`}
                        onClick={() => setEditor(location)}
                      >
                        <Pencil />
                      </IconButton>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      {editor && (
        <LocationDialog
          location={editor === "new" ? undefined : editor}
          onClose={() => setEditor(null)}
        />
      )}
    </div>
  );
}
