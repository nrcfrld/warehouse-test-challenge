import { z } from "zod";

// Quantities stay decimal strings throughout validation, persistence, and transport.
export const positiveQuantity = z
  .string()
  .regex(
    /^[0-9]{1,15}(\.[0-9]{1,3})?$/,
    "Use up to 15 integer and 3 decimal digits",
  )
  .refine((value) => /[1-9]/.test(value), "Quantity must be greater than zero")
  .transform((value) => {
    const [integer, fraction = ""] = value.split(".");
    return `${BigInt(integer)}.${fraction.padEnd(3, "0")}`;
  });

export const receiveFormSchema = z.object({
  location_id: z.string().uuid("Choose a location"),
  qty: positiveQuantity,
});

const transferFields = z.object({
  from_location_id: z.string().uuid("Choose a source location"),
  to_location_id: z.string().uuid("Choose a destination location"),
  qty: positiveQuantity,
});
const distinctLocations = (input: z.infer<typeof transferFields>) =>
  input.from_location_id !== input.to_location_id;
const distinctLocationError = {
  path: ["to_location_id"],
  message: "Choose a different destination location",
};

export const transferFormSchema = transferFields.refine(
  distinctLocations,
  distinctLocationError,
);

export const savedStockOperation = z.discriminatedUnion("operation", [
  z.object({
    operation: z.literal("receive"),
    key: z.string().uuid(),
    input: receiveFormSchema.extend({ item_id: z.string().uuid() }),
  }),
  z.object({
    operation: z.literal("transfer"),
    key: z.string().uuid(),
    input: transferFields
      .extend({ item_id: z.string().uuid() })
      .refine(distinctLocations, distinctLocationError),
  }),
]);

export type SavedStockOperation = z.infer<typeof savedStockOperation>;
