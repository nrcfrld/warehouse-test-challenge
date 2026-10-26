import { useMemo, useSyncExternalStore } from "react";
import {
  savedStockOperation,
  type SavedStockOperation,
} from "@/lib/stock-validation";

type Operation = SavedStockOperation["operation"];
type Attempt<K extends Operation> = Extract<
  SavedStockOperation,
  { operation: K }
>;
const changed = "warehouse:stock-attempt-changed";
const unavailable = "storage-unavailable";

function subscribe(listener: () => void) {
  window.addEventListener(changed, listener);
  window.addEventListener("storage", listener);
  return () => {
    window.removeEventListener(changed, listener);
    window.removeEventListener("storage", listener);
  };
}

function snapshot(key: string) {
  try {
    return sessionStorage.getItem(key) ?? "";
  } catch {
    return unavailable;
  }
}

function read<K extends Operation>(
  raw: string,
  operation: K,
  itemID: string,
): Attempt<K> | null {
  if (raw === unavailable)
    throw new Error(
      "Browser session storage is unavailable. Enable it before submitting stock requests.",
    );
  if (!raw) return null;
  try {
    const result = savedStockOperation.parse(JSON.parse(raw));
    if (result.operation === operation && result.input.item_id === itemID)
      return result as Attempt<K>;
  } catch {
    // Never silently discard an operation that might already have committed.
  }
  throw new Error(
    "The saved stock request could not be read. Review current stock balances before clearing this tab's session storage.",
  );
}

// An unresolved request is persisted before sending and is shared by the notice and dialog.
export function useStockAttempt<K extends Operation>(
  operation: K,
  itemID: string,
) {
  const storageKey = `warehouse:stock-attempt:v1:${operation}:${itemID}`;
  const raw = useSyncExternalStore(
    subscribe,
    () => snapshot(storageKey),
    () => "",
  );
  const state = useMemo(() => {
    try {
      return { attempt: read(raw, operation, itemID), error: null };
    } catch (error) {
      return { attempt: null, error: error as Error };
    }
  }, [raw, operation, itemID]);

  function prepare(input: Attempt<K>["input"]): Attempt<K> {
    const existing = read(snapshot(storageKey), operation, itemID);
    if (existing) return existing;
    const next = savedStockOperation.parse({
      operation,
      input,
      key: crypto.randomUUID(),
    }) as Attempt<K>;
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(next));
    } catch {
      throw new Error(
        "The stock request could not be saved. Enable browser session storage and try again. Nothing was sent.",
      );
    }
    window.dispatchEvent(new Event(changed));
    return next;
  }

  function resolve(key: string) {
    try {
      const current = read(snapshot(storageKey), operation, itemID);
      if (current?.key !== key) return null;
      sessionStorage.removeItem(storageKey);
    } catch {
      return new Error(
        "The result was received, but the saved request could not be cleared. Keep it and check again safely when browser session storage is available.",
      );
    }
    window.dispatchEvent(new Event(changed));
    return null;
  }

  return { ...state, prepare, resolve };
}
