import type { components } from "./api.generated";
export type Item = components["schemas"]["Item"];
export type ItemInput = components["schemas"]["ItemInput"];
export type Location = components["schemas"]["Location"];
export type StockSummary = components["schemas"]["StockSummary"];
export type ReceiveInput = components["schemas"]["ReceiveInput"];
export type Receipt = components["schemas"]["Receipt"];
export type Meta = components["schemas"]["Meta"];
export interface ItemFilters {
  search: string;
  category: string;
  page: number;
  limit: number;
}
export interface Envelope<T> {
  success: boolean;
  message: string;
  data: T;
  meta?: Meta;
  errors?: { field: string; reason: string }[];
}

export type LocationInput = components["schemas"]["LocationInput"];
export type TransferInput = components["schemas"]["TransferInput"];
export type Transfer = components["schemas"]["Transfer"];
