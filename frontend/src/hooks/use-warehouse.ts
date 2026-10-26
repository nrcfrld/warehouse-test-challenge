import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "@/services/api";
import type {
  ItemFilters,
  ItemInput,
  ReceiveInput,
  LocationInput,
  TransferInput,
} from "@/types";
export function useItems(filters: ItemFilters) {
  return useQuery({
    queryKey: ["items", filters],
    queryFn: ({ signal }) => api.items(filters, signal),
    placeholderData: keepPreviousData,
  });
}
export function useItem(id: string) {
  return useQuery({
    queryKey: ["item", id],
    queryFn: ({ signal }) => api.item(id, signal),
    enabled: Boolean(id),
  });
}
export function useStock(id: string) {
  return useQuery({
    queryKey: ["stock", id],
    queryFn: ({ signal }) => api.stock(id, signal),
    enabled: Boolean(id),
  });
}
export function useLocations(enabled = true) {
  return useQuery({
    queryKey: ["locations"],
    queryFn: ({ signal }) => api.locations(signal),
    staleTime: 300_000,
    enabled,
  });
}
export function useCategories() {
  return useQuery({
    queryKey: ["categories"],
    queryFn: ({ signal }) => api.categories(signal),
  });
}
export function useItemMutation(id?: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: ItemInput) =>
      id ? api.update(id, input) : api.create(input),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["items"] }),
        client.invalidateQueries({ queryKey: ["item", id] }),
        client.invalidateQueries({ queryKey: ["stock", id] }),
        client.invalidateQueries({ queryKey: ["categories"] }),
      ]);
    },
  });
}
export function useDeleteItem() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.remove,
    onSuccess: async (_, id) => {
      client.removeQueries({ queryKey: ["item", id] });
      client.removeQueries({ queryKey: ["stock", id] });
      await Promise.all([
        client.invalidateQueries({ queryKey: ["items"] }),
        client.invalidateQueries({ queryKey: ["categories"] }),
      ]);
    },
  });
}
export function useReceiveStock() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ input, key }: { input: ReceiveInput; key: string }) =>
      api.receive(input, key),
    onSuccess: async (_, { input }) => {
      await client.invalidateQueries({ queryKey: ["stock", input.item_id] });
    },
  });
}

export function useLocationMutation(id?: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: LocationInput) =>
      id ? api.updateLocation(id, input) : api.createLocation(input),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ["locations"] }),
        client.invalidateQueries({ queryKey: ["stock"] }),
      ]);
    },
  });
}
export function useTransferStock() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ input, key }: { input: TransferInput; key: string }) =>
      api.transfer(input, key),
    onSuccess: async (_, { input }) => {
      await client.invalidateQueries({ queryKey: ["stock", input.item_id] });
    },
  });
}
