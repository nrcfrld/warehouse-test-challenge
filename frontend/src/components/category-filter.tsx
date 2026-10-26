import { useMemo, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { Command } from "cmdk";
import { Check, ChevronsUpDown, Search, SlidersHorizontal } from "lucide-react";
import { Button } from "./ui/button";

const visibleLimit = 100;
export function CategoryFilter({
  value,
  categories,
  onValueChange,
  loading,
}: {
  value: string;
  categories: string[];
  onValueChange: (value: string) => void;
  loading: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const options = useMemo(
    () =>
      value
        ? [value, ...categories.filter((category) => category !== value)]
        : categories,
    [categories, value],
  );
  const matches = useMemo(
    () =>
      options.filter((category) =>
        category
          .toLocaleLowerCase()
          .includes(search.trim().toLocaleLowerCase()),
      ),
    [options, search],
  );
  const showAll = "all categories".includes(search.trim().toLocaleLowerCase());
  function select(category: string) {
    onValueChange(category);
    setOpen(false);
  }
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        setSearch("");
      }}
    >
      <Popover.Trigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-label="Filter by category"
          aria-expanded={open}
          disabled={loading}
          className="min-w-44 max-w-64 justify-between px-3 font-normal"
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              setSearch("");
              setOpen(true);
            }
          }}
        >
          <SlidersHorizontal
            aria-hidden="true"
            className="text-muted-foreground"
          />
          <span className="min-w-0 flex-1 truncate text-left">
            {loading ? "Loading categories…" : value || "All categories"}
          </span>
          <ChevronsUpDown
            aria-hidden="true"
            className="size-4! text-muted-foreground"
          />
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={8}
          collisionPadding={16}
          aria-label="Choose a category"
          className="z-40 w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-2xl border bg-white shadow-[0_12px_40px_#19223818]"
        >
          <Command
            label="Search categories"
            shouldFilter={false}
            defaultValue={value ? `category:${value}` : "all"}
          >
            <div className="relative border-b">
              <Search
                aria-hidden="true"
                className="pointer-events-none absolute left-4 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Command.Input
                placeholder="Search categories…"
                value={search}
                onValueChange={setSearch}
                className="h-12 rounded-none border-0 bg-transparent pl-11 pr-4 focus:ring-0"
              />
            </div>
            <Command.List
              label="Categories"
              className="max-h-64 overflow-y-auto overscroll-contain p-1.5"
            >
              <Command.Empty className="px-4 py-8 text-center text-sm text-muted-foreground">
                No categories found.
              </Command.Empty>
              {showAll && (
                <Command.Item
                  value="all"
                  onSelect={() => select("")}
                  className="category-option"
                >
                  <span className="flex-1">All categories</span>
                  {!value && (
                    <Check aria-hidden="true" className="size-4 text-primary" />
                  )}
                </Command.Item>
              )}
              {matches.slice(0, visibleLimit).map((category) => (
                <Command.Item
                  key={category}
                  value={`category:${category}`}
                  onSelect={() => select(category)}
                  className="category-option"
                >
                  <span className="min-w-0 flex-1 break-words">{category}</span>
                  {value === category && (
                    <Check
                      aria-hidden="true"
                      className="size-4 shrink-0 text-primary"
                    />
                  )}
                </Command.Item>
              ))}
            </Command.List>
            {matches.length > visibleLimit && (
              <p
                role="status"
                className="border-t px-4 py-3 text-xs leading-5 text-muted-foreground"
              >
                Showing {visibleLimit} of {matches.length} categories. Type to
                narrow the list.
              </p>
            )}
          </Command>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
