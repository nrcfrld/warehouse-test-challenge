import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse, delay } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { TooltipProvider } from "@/components/ui/tooltip";
import ItemsPage from "@/pages/items";
import ItemFormPage from "@/pages/item-form";
import { TransferDialog } from "@/components/transfer-dialog";
import { LocationDialog } from "@/components/location-dialog";
import { ReceiveDialog } from "@/components/receive-dialog";
import { quantity } from "@/lib/utils";
const item = {
  id: "20000000-0000-4000-8000-000000000001",
  sku: "PKG-001",
  name: "Shipping box",
  category: "Packaging",
  unit: "pcs",
  created_at: "2026-01-01T00:00:00Z",
};
const location = {
  id: "10000000-0000-4000-8000-000000000001",
  code: "A-01",
  zone: "Main",
  type: "Shelf",
};
const success = (data: unknown) =>
  HttpResponse.json({ success: true, message: "Success", data });
const server = setupServer(
  http.get("*/api/v1/categories", () => success(["Packaging"])),
  http.get("*/api/v1/locations", () => success([location])),
  http.get("*/api/v1/items", () =>
    HttpResponse.json({
      success: true,
      message: "Success",
      data: [item],
      meta: { page: 1, limit: 10, total: 1 },
    }),
  ),
);
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
function setup(children: React.ReactNode, entry = "/items") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider delayDuration={0}>
        <MemoryRouter
          initialEntries={[entry]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          {children}
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
}
describe("warehouse interactions", () => {
  it("debounces search and keeps keyboard focus after results arrive", async () => {
    const requests: string[] = [];
    server.use(
      http.get("*/api/v1/items", ({ request }) => {
        requests.push(new URL(request.url).searchParams.get("search") || "");
        return HttpResponse.json({
          success: true,
          message: "OK",
          data: [item],
          meta: { page: 1, limit: 10, total: 1 },
        });
      }),
    );
    setup(<ItemsPage />);
    await screen.findByText("Shipping box");
    const input = screen.getByRole("textbox", { name: "Search items" });
    const user = userEvent.setup();
    await user.type(input, "cotton");
    expect(requests).toEqual([""]);
    await waitFor(() => expect(requests).toContain("cotton"));
    expect(input).toHaveFocus();
    expect(requests).toEqual(["", "cotton"]);
  });
  it("shows a retryable API failure", async () => {
    server.use(
      http.get("*/api/v1/items", () =>
        HttpResponse.json(
          { success: false, message: "Database unavailable", errors: [] },
          { status: 503 },
        ),
      ),
    );
    setup(<ItemsPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Database unavailable",
    );
    expect(screen.getByRole("button", { name: "Try again" })).toBeVisible();
  });
  it("validates required fields and maps duplicate SKU errors to the field", async () => {
    let submitted = 0;
    server.use(
      http.post("*/api/v1/items", () => {
        submitted++;
        return HttpResponse.json(
          {
            success: false,
            message: "SKU already exists",
            errors: [{ field: "sku", reason: "Duplicate SKU" }],
          },
          { status: 409 },
        );
      }),
    );
    setup(
      <Routes>
        <Route path="/items/new" element={<ItemFormPage />} />
      </Routes>,
      "/items/new",
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Create item" }));
    expect(await screen.findByText("Name is required")).toBeVisible();
    expect(submitted).toBe(0);
    for (const [label, value] of [
      ["Item name", "New item"],
      ["SKU", "PKG-001"],
      ["Category", "Packaging"],
      ["Unit of measure", "pcs"],
    ])
      await user.type(screen.getByLabelText(label), value);
    await user.click(screen.getByRole("button", { name: "Create item" }));
    expect(await screen.findByText("Duplicate SKU")).toBeVisible();
    expect(submitted).toBe(1);
  });
  it("keeps the idempotency key after an uncertain receipt and disables duplicate submits", async () => {
    const keys: (string | null)[] = [];
    const quantities: unknown[] = [];
    server.use(
      http.post("*/api/v1/stock/receive", async ({ request }) => {
        keys.push(request.headers.get("Idempotency-Key"));
        quantities.push(((await request.json()) as { qty: string }).qty);
        await delay(120);
        return HttpResponse.json(
          { success: false, message: "Unavailable", errors: [] },
          { status: 503 },
        );
      }),
    );
    setup(
      <ReceiveDialog open onOpenChange={() => {}} itemID={item.id} unit="kg" />,
    );
    const user = userEvent.setup();
    await screen.findByRole("option", { name: "A-01 · Main" });
    await user.selectOptions(
      screen.getByLabelText("Storage location"),
      location.id,
    );
    await user.type(screen.getByLabelText(/Quantity/), "0.100");
    const submit = screen.getByRole("button", { name: "Receive stock" });
    await user.click(submit);
    expect(submit).toBeDisabled();
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);
    await waitFor(() => expect(keys).toHaveLength(2));
    expect(keys[0]).toBeTruthy();
    expect(keys[0]).toBe(keys[1]);
    expect(quantities).toEqual(["0.100", "0.100"]);
    await waitFor(() => expect(submit).toBeEnabled());
  });
  it("validates location metadata and displays duplicate code errors", async () => {
    let submitted = 0;
    server.use(
      http.post("*/api/v1/locations", () => {
        submitted++;
        return HttpResponse.json(
          {
            success: false,
            message: "Location code already exists",
            errors: [{ field: "code", reason: "Duplicate location code" }],
          },
          { status: 409 },
        );
      }),
    );
    setup(<LocationDialog onClose={() => {}} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Create location" }));
    expect(await screen.findByText("Location code is required")).toBeVisible();
    expect(submitted).toBe(0);
    await user.type(screen.getByLabelText("Location code"), "A-01");
    await user.type(screen.getByLabelText("Zone"), "Main");
    await user.type(screen.getByLabelText("Type"), "Shelf");
    await user.click(screen.getByRole("button", { name: "Create location" }));
    expect(await screen.findByText("Duplicate location code")).toBeVisible();
    expect(submitted).toBe(1);
  });
  it("keeps a transfer retry key and maps insufficient stock to quantity", async () => {
    const destination = {
      ...location,
      id: "10000000-0000-4000-8000-000000000002",
      code: "B-01",
    };
    const keys: (string | null)[] = [];
    server.use(
      http.get("*/api/v1/locations", () => success([location, destination])),
      http.post("*/api/v1/stock/transfer", async ({ request }) => {
        keys.push(request.headers.get("Idempotency-Key"));
        await delay(100);
        if (keys.length === 1)
          return HttpResponse.json(
            { success: false, message: "Unavailable", errors: [] },
            { status: 503 },
          );
        return HttpResponse.json(
          {
            success: false,
            message: "Insufficient stock",
            errors: [
              {
                field: "qty",
                reason: "Insufficient stock at the source location",
              },
            ],
          },
          { status: 409 },
        );
      }),
    );
    setup(
      <TransferDialog
        open
        onOpenChange={() => {}}
        itemID={item.id}
        unit="kg"
      />,
    );
    const user = userEvent.setup();
    await screen.findAllByRole("option", { name: "B-01 · Main" });
    await user.selectOptions(
      screen.getByLabelText("From location"),
      location.id,
    );
    expect(screen.getByLabelText("To location")).not.toContainHTML(
      `value="${location.id}"`,
    );
    await user.selectOptions(
      screen.getByLabelText("To location"),
      destination.id,
    );
    await user.type(screen.getByLabelText(/Quantity/), "0.100");
    const submit = screen.getByRole("button", { name: "Transfer stock" });
    await user.click(submit);
    expect(submit).toBeDisabled();
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);
    expect(
      await screen.findByText("Insufficient stock at the source location"),
    ).toBeVisible();
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBeTruthy();
    expect(keys[1]).toBe(keys[0]);
  });
  it("searches categories beyond the initial list, selects with the keyboard, and resets pagination", async () => {
    const categories = Array.from(
      { length: 250 },
      (_, i) => `Category ${String(i).padStart(3, "0")}`,
    );
    const requests: URL[] = [];
    server.use(
      http.get("*/api/v1/categories", () => success(categories)),
      http.get("*/api/v1/items", ({ request }) => {
        const url = new URL(request.url);
        requests.push(url);
        return HttpResponse.json({
          success: true,
          message: "OK",
          data: [item],
          meta: {
            page: Number(url.searchParams.get("page")),
            limit: 10,
            total: 40,
          },
        });
      }),
    );
    setup(<ItemsPage />, "/items?page=3");
    await screen.findByText("Shipping box");
    const user = userEvent.setup();
    const trigger = screen.getByRole("combobox", {
      name: "Filter by category",
    });
    await waitFor(() => expect(trigger).toBeEnabled());
    await user.click(trigger);
    const search = screen.getByRole("combobox", { name: "Search categories" });
    expect(search).toHaveFocus();
    expect(screen.getAllByRole("option")).toHaveLength(101);
    expect(screen.getByRole("status")).toHaveTextContent(
      "Showing 100 of 250 categories",
    );
    await user.type(search, "category 249");
    expect(
      await screen.findByRole("option", { name: "Category 249" }),
    ).toBeVisible();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(trigger).toHaveTextContent("Category 249");
    await waitFor(() =>
      expect(requests.at(-1)?.searchParams.get("category")).toBe(
        "Category 249",
      ),
    );
    expect(requests.at(-1)?.searchParams.get("page")).toBe("1");
    await user.click(trigger);
    expect(screen.getByRole("option", { name: "Category 249" })).toBeVisible();
    await user.click(screen.getByRole("option", { name: "All categories" }));
    await waitFor(() =>
      expect(requests.at(-1)?.searchParams.get("category")).toBe(""),
    );
    expect(trigger).toHaveTextContent("All categories");
  });
  it("shows empty category results and closes with Escape without changing the filter", async () => {
    setup(<ItemsPage />, "/items?category=Archived");
    await screen.findByText("Shipping box");
    const user = userEvent.setup();
    const trigger = screen.getByRole("combobox", {
      name: "Filter by category",
    });
    await waitFor(() => expect(trigger).toBeEnabled());
    await user.click(trigger);
    expect(screen.getByRole("option", { name: "Archived" })).toBeVisible();
    await user.type(
      screen.getByRole("combobox", { name: "Search categories" }),
      "no-such-category",
    );
    expect(await screen.findByText("No categories found.")).toBeVisible();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(trigger).toHaveTextContent("Archived");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    await user.click(trigger);
    expect(
      screen.getByRole("combobox", { name: "Search categories" }),
    ).toHaveValue("");
  });
  it("formats large quantities without losing digits", () => {
    expect(quantity("999999999999999.999")).toBe("999,999,999,999,999.999");
    expect(quantity("0.300")).toBe("0.3");
  });
});
