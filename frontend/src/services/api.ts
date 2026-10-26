import type {
  Envelope,
  LocationInput,
  TransferInput,
  Transfer,
  Item,
  ItemFilters,
  ItemInput,
  Location,
  ReceiveInput,
  Receipt,
  StockSummary,
} from "@/types";
export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
    public fields: { field: string; reason: string }[] = [],
  ) {
    super(message);
    this.name = "APIError";
  }
}
async function request<T>(
  path: string,
  init?: RequestInit,
): Promise<Envelope<T>> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      ...init,
      headers: {
        ...init?.headers,
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
      },
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError")
      throw error;
    throw new APIError(
      0,
      "Unable to reach the server. Check your connection and try again.",
    );
  }
  let body: Envelope<T>;
  try {
    body = (await response.json()) as Envelope<T>;
  } catch {
    throw new APIError(
      response.status,
      "The server returned an unreadable response. Please try again.",
    );
  }
  if (!response.ok || !body.success)
    throw new APIError(
      response.status,
      body.message || "Request failed",
      body.errors,
    );
  return body;
}
export const api = {
  items: (filters: ItemFilters, signal?: AbortSignal) =>
    request<Item[]>(
      `/items?${new URLSearchParams(Object.entries(filters).map(([key, value]) => [key, String(value)]))}`,
      { signal },
    ),
  item: (id: string, signal?: AbortSignal) =>
    request<Item>(`/items/${id}`, { signal }),
  create: (input: ItemInput) =>
    request<Item>("/items", { method: "POST", body: JSON.stringify(input) }),
  update: (id: string, input: ItemInput) =>
    request<Item>(`/items/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  remove: (id: string) =>
    request<{ id: string }>(`/items/${id}`, { method: "DELETE" }),
  stock: (id: string, signal?: AbortSignal) =>
    request<StockSummary>(`/stock/${id}`, { signal }),
  locations: (signal?: AbortSignal) =>
    request<Location[]>("/locations", { signal }),
  createLocation: (input: LocationInput) =>
    request<Location>("/locations", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateLocation: (id: string, input: LocationInput) =>
    request<Location>(`/locations/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  transfer: (input: TransferInput, key: string) =>
    request<Transfer>("/stock/transfer", {
      method: "POST",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify(input),
    }),
  categories: (signal?: AbortSignal) =>
    request<string[]>("/categories", { signal }),
  receive: (input: ReceiveInput, key: string) =>
    request<Receipt>("/stock/receive", {
      method: "POST",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify(input),
    }),
};
