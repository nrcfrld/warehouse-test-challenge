import { afterEach, expect, it, vi } from "vitest";
import { z } from "zod";
import { createIdempotencyKey } from "@/lib/idempotency-key";

afterEach(() => vi.unstubAllGlobals());

it.each([
  [0, "00000000-0000-4000-8000-000000000000"],
  [255, "ffffffff-ffff-4fff-bfff-ffffffffffff"],
] as const)(
  "creates a valid UUID without randomUUID (random byte %i)",
  (byte, expected) => {
    const getRandomValues = vi.fn((values: Uint8Array) => values.fill(byte));
    vi.stubGlobal("crypto", { getRandomValues });

    const key = createIdempotencyKey();

    expect(key).toBe(expected);
    expect(z.string().uuid().parse(key)).toBe(key);
    expect(getRandomValues).toHaveBeenCalledOnce();
    expect(getRandomValues.mock.calls[0][0]).toHaveLength(16);
  },
);
