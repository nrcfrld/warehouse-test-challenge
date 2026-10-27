import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { ReceiveDialog } from "@/components/receive-dialog";
import { TransferDialog } from "@/components/transfer-dialog";
import { PendingStockNotice } from "@/components/pending-stock-notice";
import { TooltipProvider } from "@/components/ui/tooltip";

const itemID = "20000000-0000-4000-8000-000000000001";
const locationID = "10000000-0000-4000-8000-000000000001";
const destinationID = "10000000-0000-4000-8000-000000000002";
const server = setupServer(
  http.get("*/api/v1/locations", () =>
    HttpResponse.json({
      success: true,
      message: "OK",
      data: [
        { id: locationID, code: "A", zone: "Main", type: "Shelf" },
        { id: destinationID, code: "B", zone: "Main", type: "Shelf" },
      ],
    }),
  ),
);
beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
afterAll(() => server.close());

function mount(operation: "receive" | "transfer") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <PendingStockNotice
          itemID={itemID}
          onReceive={() => {}}
          onTransfer={() => {}}
        />
        {operation === "receive" ? (
          <ReceiveDialog
            open
            onOpenChange={() => {}}
            itemID={itemID}
            unit="kg"
          />
        ) : (
          <TransferDialog
            open
            onOpenChange={() => {}}
            itemID={itemID}
            unit="kg"
          />
        )}
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

async function fill(operation: "receive" | "transfer") {
  const user = userEvent.setup();
  await screen.findAllByRole("option", { name: "A · Main" });
  if (operation === "receive") {
    await user.selectOptions(
      screen.getByLabelText("Storage location"),
      locationID,
    );
  } else {
    await user.selectOptions(
      screen.getByLabelText("From location"),
      locationID,
    );
    await user.selectOptions(
      screen.getByLabelText("To location"),
      destinationID,
    );
  }
  await user.type(screen.getByLabelText(/Quantity/), "0001.2");
  await user.click(
    screen.getByRole("button", {
      name: operation === "receive" ? "Receive stock" : "Transfer stock",
    }),
  );
}

it.each(["receive", "transfer"] as const)(
  "recovers a %s with the same key and payload after remount, even when lookups fail",
  async (operation) => {
    // Public HTTP exposes getRandomValues, but not randomUUID.
    vi.stubGlobal("crypto", {
      getRandomValues: globalThis.crypto.getRandomValues.bind(
        globalThis.crypto,
      ),
    });
    const requests: { key: string | null; input: unknown }[] = [];
    const committed = new Set<string | null>();
    server.use(
      http.post(`*/api/v1/stock/${operation}`, async ({ request }) => {
        const key = request.headers.get("Idempotency-Key");
        requests.push({ key, input: await request.json() });
        committed.add(key);
        if (requests.length === 1) return HttpResponse.error(); // Commit succeeded; its response was lost.
        return HttpResponse.json({
          success: true,
          message: "Confirmed",
          data: {},
        });
      }),
    );
    const first = mount(operation);
    await fill(operation);
    const label =
      operation === "receive" ? "Check saved receipt" : "Check saved transfer";
    await waitFor(() =>
      expect(screen.getByRole("button", { name: label })).toBeEnabled(),
    );
    expect(requests).toHaveLength(1);
    expect(sessionStorage.length).toBe(1);
    first.unmount();
    server.use(
      http.get("*/api/v1/locations", () =>
        HttpResponse.json(
          { success: false, message: "Unavailable", errors: [] },
          { status: 503 },
        ),
      ),
    );
    mount(operation);
    expect(screen.getByLabelText(/Quantity/)).toHaveValue("1.200");
    expect(screen.getByLabelText(/Quantity/)).toBeDisabled();
    expect(requests).toHaveLength(1); // Recovery never sends an automatic mutation.
    await userEvent.setup().click(screen.getByRole("button", { name: label }));
    await waitFor(() => expect(requests).toHaveLength(2));
    await waitFor(() => expect(sessionStorage.length).toBe(0));
    expect(requests[1]).toEqual(requests[0]);
    expect(committed.size).toBe(1);
  },
);

it("clears a definitive rejection and allows a corrected request with a new key", async () => {
  const keys: (string | null)[] = [];
  server.use(
    http.post("*/api/v1/stock/receive", ({ request }) => {
      keys.push(request.headers.get("Idempotency-Key"));
      return HttpResponse.json(
        {
          success: false,
          message: "Rejected",
          errors: [
            { field: "qty", reason: "Quantity exceeds the supported balance" },
          ],
        },
        { status: 409 },
      );
    }),
  );
  mount("receive");
  await fill("receive");
  await screen.findByText("Quantity exceeds the supported balance");
  expect(sessionStorage.length).toBe(0);
  const qty = screen.getByLabelText(/Quantity/);
  expect(qty).toBeEnabled();
  const user = userEvent.setup();
  await user.clear(qty);
  await user.type(qty, "1");
  await user.click(screen.getByRole("button", { name: "Receive stock" }));
  await waitFor(() => expect(keys).toHaveLength(2));
  expect(keys[1]).not.toBe(keys[0]);
});

it("blocks a new submission when the pending record cannot be read", async () => {
  sessionStorage.setItem(
    `warehouse:stock-attempt:v1:receive:${itemID}`,
    "broken JSON",
  );
  mount("receive");
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "saved stock request could not be read",
  );
  expect(
    screen.queryByRole("button", { name: "Receive stock" }),
  ).not.toBeInTheDocument();
  expect(sessionStorage.length).toBe(1);
});

it("never sends a stock request if persisting the key fails", async () => {
  let sent = 0;
  server.use(
    http.post("*/api/v1/stock/receive", () => {
      sent++;
      return HttpResponse.error();
    }),
  );
  const write = vi
    .spyOn(Storage.prototype, "setItem")
    .mockImplementation(() => {
      throw new Error("Storage is full");
    });
  mount("receive");
  await fill("receive");
  await waitFor(() => expect(write).toHaveBeenCalled());
  expect(sent).toBe(0);
  expect(sessionStorage.length).toBe(0);
});
