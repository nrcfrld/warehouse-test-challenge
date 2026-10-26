import { useRef } from "react";
import { Search, X } from "lucide-react";
import { IconButton } from "./icon-button";

export function SearchField({
  label,
  placeholder,
  value,
  onValueChange,
  maxLength,
}: {
  label: string;
  placeholder: string;
  value: string;
  onValueChange: (value: string) => void;
  maxLength?: number;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <div className="relative w-full max-w-sm">
      <Search
        aria-hidden="true"
        className="pointer-events-none absolute left-3.5 top-1/2 size-[18px] -translate-y-1/2 text-muted-foreground"
      />
      <input
        ref={input}
        aria-label={label}
        placeholder={placeholder}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
        maxLength={maxLength}
        className="h-11 border-transparent bg-muted/80 pl-11 pr-11 focus:bg-white"
      />
      {value && (
        <IconButton
          type="button"
          label="Clear search"
          className="absolute right-0 top-0"
          onClick={() => {
            onValueChange("");
            input.current?.focus();
          }}
        >
          <X />
        </IconButton>
      )}
    </div>
  );
}
