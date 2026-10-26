import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { randomUUID } from "node:crypto";

for (const operation of ["receive", "transfer"] as const) {
  test(`recover ${operation} after a committed response is lost and the page is reloaded`, async ({
    page,
    request,
  }) => {
    const created = await request.post("/api/v1/items", {
      data: {
        sku: `RECOVERY-${randomUUID()}`,
        name: "Recovery verification",
        category: "E2E recovery",
        unit: "kg",
      },
    });
    expect(created.status()).toBe(201);
    const item = (await created.json()).data;
    const source = "10000000-0000-4000-8000-000000000001";
    const destination = "10000000-0000-4000-8000-000000000002";
    const keys: (string | undefined)[] = [];
    let committed = false;
    try {
      if (operation === "transfer") {
        const seeded = await request.post("/api/v1/stock/receive", {
          data: { item_id: item.id, location_id: source, qty: "1" },
          headers: { "Idempotency-Key": randomUUID() },
        });
        expect(seeded.ok()).toBe(true);
      }
      await page.route(`**/api/v1/stock/${operation}`, async (route) => {
        keys.push(route.request().headers()["idempotency-key"]);
        const result = await route.fetch();
        expect(result.ok()).toBe(true);
        if (keys.length === 1) {
          committed = true;
          await route.abort("connectionfailed");
        } else {
          expect(result.headers()["idempotency-replayed"]).toBe("true");
          await route.fulfill({ response: result });
        }
      });
      await page.goto(`/items/${item.id}`);
      const title =
        operation === "receive" ? "Receive stock" : "Transfer stock";
      await page.getByRole("button", { name: title, exact: true }).click();
      let dialog = page.getByRole("dialog", { name: title, exact: true });
      if (operation === "receive") {
        await dialog.getByLabel("Storage location").selectOption(source);
      } else {
        await dialog.getByLabel("From location").selectOption(source);
        await dialog.getByLabel("To location").selectOption(destination);
      }
      await dialog.getByLabel(/Quantity/).fill("0.100");
      await dialog.getByRole("button", { name: title, exact: true }).click();
      const check =
        operation === "receive"
          ? "Check saved receipt"
          : "Check saved transfer";
      await expect(dialog.getByRole("button", { name: check })).toBeEnabled();
      await expect.poll(() => committed).toBe(true);
      await expect(page.getByText(/outcome is unknown/)).toBeVisible();
      await page.reload();
      await page
        .getByRole("button", {
          name: operation === "receive" ? "Check receipt" : "Check transfer",
          exact: true,
        })
        .click();
      dialog = page.getByRole("dialog", { name: title, exact: true });
      await expect(dialog.getByLabel(/Quantity/)).toHaveValue("0.100");
      await expect(dialog.getByLabel(/Quantity/)).toBeDisabled();
      expect(keys).toHaveLength(1);
      await dialog.getByRole("button", { name: check }).click();
      await expect(dialog).not.toBeVisible();
      expect(keys).toHaveLength(2);
      expect(keys[0]).toBeTruthy();
      expect(keys[1]).toBe(keys[0]);
      const stock = (
        await (await request.get(`/api/v1/stock/${item.id}`)).json()
      ).data;
      expect(stock.total).toBe(operation === "receive" ? "0.100" : "1.000");
      const balance = (id: string) =>
        stock.locations.find(
          (entry: { location: { id: string } }) => entry.location.id === id,
        ).qty;
      expect(balance(source)).toBe(operation === "receive" ? "0.100" : "0.900");
      if (operation === "transfer") expect(balance(destination)).toBe("0.100");
      expect(
        await page.evaluate(
          () =>
            Object.keys(sessionStorage).filter((key) =>
              key.startsWith("warehouse:stock-attempt:v1:"),
            ).length,
        ),
      ).toBe(0);
    } finally {
      await request.delete(`/api/v1/items/${item.id}`);
    }
  });
}

test("create, find, edit, receive exact stock, and soft-delete an item", async ({
  page,
  request,
}) => {
  const suffix = randomUUID().slice(0, 8);
  const sku = `E2E-${suffix}`.toUpperCase();
  const name = `Cotton canvas ${suffix}`;
  await page.goto("/items");
  await expect(page.getByRole("heading", { name: /Inventory/ })).toBeVisible();
  await page.getByRole("link", { name: "Add item", exact: true }).focus();
  await expect(page.getByRole("tooltip", { name: "Add item" })).toBeVisible();
  await page.keyboard.press("Enter");
  await page.getByLabel("Item name", { exact: true }).fill(name);
  await page.getByLabel("SKU", { exact: true }).fill(sku);
  await page.getByLabel("Category", { exact: true }).fill("E2E materials");
  await page.getByLabel("Unit of measure", { exact: true }).fill("kg");
  await page.getByRole("button", { name: "Create item" }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  const id = page.url().split("/").pop()!;
  await page.getByRole("link", { name: "Edit item", exact: true }).click();
  await page.getByLabel("Item name", { exact: true }).fill(`${name} updated`);
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(
    page.getByRole("heading", { name: `${name} updated`, exact: true }),
  ).toBeVisible();
  for (const qty of ["0.100", "0.200"]) {
    await page
      .getByRole("button", { name: "Receive stock", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "Receive stock" });
    await expect(dialog).toBeVisible();
    await dialog
      .getByLabel("Storage location")
      .selectOption("10000000-0000-4000-8000-000000000001");
    await dialog.getByLabel(/Quantity/).fill(qty);
    await dialog
      .getByRole("button", { name: "Receive stock", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("button", { name: "Receive stock", exact: true }),
    ).toBeFocused();
  }
  await expect(page.getByTestId("stock-total")).toHaveText("0.3kg");
  const balance = await request.get(`/api/v1/stock/${id}`);
  expect((await balance.json()).data.total).toBe("0.300");
  const accessibility = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  expect(accessibility.violations).toEqual([]);
  await page.getByRole("link", { name: "Back to inventory" }).click();
  await page.getByRole("textbox", { name: "Search items" }).fill(sku);
  await page.getByRole("combobox", { name: "Filter by category" }).click();
  await page
    .getByRole("combobox", { name: "Search categories" })
    .fill("E2E materials");
  await page
    .getByRole("option", { name: "E2E materials", exact: true })
    .click();
  await expect(
    page.getByRole("link", { name: `${name} updated`, exact: true }),
  ).toBeVisible();
  await expect(page).toHaveURL(/search=/);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page
    .getByRole("button", { name: `Delete ${name} updated`, exact: true })
    .click();
  const confirm = page.getByRole("alertdialog");
  await expect(confirm).toContainText(
    "Existing stock and transaction history will be preserved",
  );
  await expect(confirm.getByRole("button", { name: "Cancel" })).toBeFocused();
  await confirm.getByRole("button", { name: "Delete item" }).click();
  await expect(
    page.getByRole("heading", { name: "No matching items" }),
  ).toBeVisible();
  expect((await request.get(`/api/v1/items/${id}`)).status()).toBe(404);
  const duplicate = await request.post("/api/v1/items", {
    data: { sku, name: "Duplicate", category: "E2E materials", unit: "kg" },
  });
  expect(duplicate.status()).toBe(409);
});
test("supports empty filters, missing items, and accessible inventory controls", async ({
  page,
}) => {
  await page.goto("/items?search=does-not-exist-987654321");
  await expect(
    page.getByRole("heading", { name: "No matching items" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Clear filters", exact: true })
    .click();
  await expect(page.getByRole("textbox", { name: "Search items" })).toHaveValue(
    "",
  );
  await page.screenshot({
    path: `test-results/inventory-${test.info().project.name}.png`,
    fullPage: true,
  });
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  expect(results.violations).toEqual([]);
  await page.goto(`/items/${randomUUID()}`);
  await expect(
    page.getByRole("heading", { name: "Item not found" }),
  ).toBeVisible();
});

test("pagination returns to the previous page after deleting the last row", async ({
  page,
  request,
}) => {
  const category = `Pagination ${randomUUID().slice(0, 8)}`;
  const items: { id: string; name: string }[] = [];
  try {
    for (let i = 0; i < 11; i++) {
      const response = await request.post("/api/v1/items", {
        data: {
          sku: randomUUID(),
          name: `${category} item ${i}`,
          category,
          unit: "pcs",
        },
      });
      expect(response.status()).toBe(201);
      items.push((await response.json()).data);
    }
    await page.goto(`/items?category=${encodeURIComponent(category)}`);
    await expect(page.getByText("Page 1 of 2", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Next page", exact: true }).click();
    await expect(page.getByText("Page 2 of 2", { exact: true })).toBeVisible();
    await page
      .getByRole("button", { name: `Delete ${items[0].name}`, exact: true })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Delete item", exact: true })
      .click();
    await expect(page.getByText("Page 1 of 1", { exact: true })).toBeVisible();
    await expect(page).toHaveURL(/page=1/);
  } finally {
    for (const item of items) await request.delete(`/api/v1/items/${item.id}`);
  }
});

test("create and edit a location, then transfer stock without changing its total", async ({
  page,
  request,
}) => {
  const suffix = randomUUID().slice(0, 8).toUpperCase();
  const code = `E2E-${suffix}`;
  await page.goto("/items");
  await page.getByLabel("Locations", { exact: true }).click();
  await expect(page.getByLabel("Locations", { exact: true })).toHaveCSS(
    "display",
    "flex",
  );
  await expect(page.getByLabel("Locations", { exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByLabel("Locations", { exact: true })).toHaveClass(
    /bg-blue-50/,
  );
  await expect(page.getByLabel("Inventory", { exact: true })).not.toHaveClass(
    /bg-blue-50/,
  );

  await expect(
    page.getByRole("heading", { name: "Locations", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Add location", exact: true }).click();
  let dialog = page.getByRole("dialog", { name: "Add location", exact: true });
  await dialog.getByLabel("Location code").fill(code.toLowerCase());
  await dialog.getByLabel("Zone", { exact: true }).fill("E2E receiving");
  await dialog.getByLabel("Type", { exact: true }).fill("Bin");
  await dialog.getByRole("button", { name: "Create location" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByRole("button", { name: "Add location", exact: true }),
  ).toBeFocused();
  await page.getByRole("textbox", { name: "Search locations" }).fill(code);
  await page.getByRole("button", { name: `Edit ${code}`, exact: true }).focus();
  await expect(
    page.getByRole("tooltip", { name: `Edit ${code}`, exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Enter");
  dialog = page.getByRole("dialog", { name: "Edit location", exact: true });
  await dialog.getByLabel("Zone", { exact: true }).fill("E2E storage");
  await dialog.getByRole("button", { name: "Save changes" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByRole("cell", { name: "E2E storage", exact: true }),
  ).toBeVisible();
  expect(
    (
      await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
        .analyze()
    ).violations,
  ).toEqual([]);
  await page.screenshot({
    path: `test-results/locations-${test.info().project.name}.png`,
    fullPage: true,
  });
  const locations = (await (await request.get("/api/v1/locations")).json())
    .data as { id: string; code: string }[];
  const destination = locations.find((l) => l.code === code)!;
  const source = "10000000-0000-4000-8000-000000000001";
  await page.getByLabel("Inventory", { exact: true }).click();
  await expect(page).toHaveURL(/\/items$/);
  const created = await request.post("/api/v1/items", {
    data: {
      sku: `TRANSFER-${suffix}`,
      name: `Transfer sample ${suffix}`,
      category: "E2E transfers",
      unit: "kg",
    },
  });
  expect(created.status()).toBe(201);
  const item = (await created.json()).data;
  try {
    const receipt = await request.post("/api/v1/stock/receive", {
      data: { item_id: item.id, location_id: source, qty: "1.250" },
      headers: { "Idempotency-Key": randomUUID() },
    });
    expect(receipt.ok()).toBe(true);
    await page.goto(`/items/${item.id}`);
    await expect(page.getByTestId("stock-total")).toHaveText("1.25kg");
    const open = page.getByRole("button", {
      name: "Transfer stock",
      exact: true,
    });
    await open.focus();
    await expect(
      page.getByRole("tooltip", { name: "Transfer stock", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Enter");
    dialog = page.getByRole("dialog", { name: "Transfer stock", exact: true });
    await dialog.getByLabel("From location").selectOption(source);
    await dialog.getByLabel("To location").selectOption(destination.id);
    await dialog.getByLabel(/Quantity/).fill("2.000");
    await dialog
      .getByRole("button", { name: "Transfer stock", exact: true })
      .click();
    await expect(
      dialog.getByText("Insufficient stock at the source location", {
        exact: true,
      }),
    ).toBeVisible();
    await dialog.getByLabel(/Quantity/).fill("0.100");
    expect(
      (
        await new AxeBuilder({ page })
          .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
          .analyze()
      ).violations,
    ).toEqual([]);
    await page.screenshot({
      path: `test-results/transfer-${test.info().project.name}.png`,
      fullPage: true,
    });
    await dialog
      .getByRole("button", { name: "Transfer stock", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(open).toBeFocused();
    await expect(page.getByTestId("stock-total")).toHaveText("1.25kg");
    await expect(page.getByRole("row").filter({ hasText: code })).toContainText(
      "0.1",
    );
    const stock = (await (await request.get(`/api/v1/stock/${item.id}`)).json())
      .data;
    expect(stock.total).toBe("1.250");
    expect(
      stock.locations.find(
        (s: { location: { id: string } }) => s.location.id === source,
      ).qty,
    ).toBe("1.150");
    expect(
      stock.locations.find(
        (s: { location: { id: string } }) => s.location.id === destination.id,
      ).qty,
    ).toBe("0.100");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  } finally {
    await request.delete(`/api/v1/items/${item.id}`);
  }
});

test("consistent search fields and searchable category keyboard navigation", async ({
  page,
}) => {
  const categories = Array.from(
    { length: 250 },
    (_, i) => `Category ${String(i).padStart(3, "0")}`,
  );
  await page.route("**/api/v1/categories", (route) =>
    route.fulfill({
      json: {
        success: true,
        message: "Categories retrieved successfully",
        data: categories,
      },
    }),
  );
  await page.goto("/items");
  const itemSearch = page.getByRole("textbox", {
    name: "Search items",
    exact: true,
  });
  await expect(itemSearch).toBeVisible();
  const styles = await itemSearch.evaluate((element) => {
    const style = getComputedStyle(element);
    return [
      style.height,
      style.backgroundColor,
      style.borderRadius,
      style.borderTopColor,
      style.paddingLeft,
      style.paddingRight,
      style.fontSize,
    ];
  });
  const trigger = page.getByRole("combobox", {
    name: "Filter by category",
    exact: true,
  });
  await trigger.focus();
  await page.keyboard.press("ArrowDown");
  const search = page.getByRole("combobox", {
    name: "Search categories",
    exact: true,
  });
  await expect(search).toBeFocused();
  await expect(page.getByRole("option")).toHaveCount(101);
  await search.fill("Category 249");
  await expect(
    page.getByRole("option", { name: "Category 249", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: `test-results/category-filter-${test.info().project.name}.png`,
    fullPage: true,
  });
  expect(
    (
      await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
        .analyze()
    ).violations,
  ).toEqual([]);
  await page.keyboard.press("Enter");
  await expect(trigger).toBeFocused();
  await expect(trigger).toContainText("Category 249");
  await expect(page).toHaveURL(/category=Category\+249/);
  await trigger.click();
  await expect(
    page.getByRole("option", { name: "Category 249", exact: true }),
  ).toBeVisible();
  await search.fill("does-not-exist");
  await expect(
    page.getByText("No categories found.", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await trigger.click();
  await page
    .getByRole("option", { name: "All categories", exact: true })
    .click();
  await expect(trigger).toContainText("All categories");
  await page.getByLabel("Locations", { exact: true }).click();
  const locationSearch = page.getByRole("textbox", {
    name: "Search locations",
    exact: true,
  });
  await expect(locationSearch).toBeVisible();
  const locationStyles = await locationSearch.evaluate((element) => {
    const style = getComputedStyle(element);
    return [
      style.height,
      style.backgroundColor,
      style.borderRadius,
      style.borderTopColor,
      style.paddingLeft,
      style.paddingRight,
      style.fontSize,
    ];
  });
  expect(locationStyles).toEqual(styles);
  await locationSearch.fill("no-such-location");
  await expect(
    page.getByRole("heading", { name: "No matching locations", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Clear search", { exact: true }).click();
  await expect(locationSearch).toHaveValue("");
  await expect(locationSearch).toBeFocused();
  await page.screenshot({
    path: `test-results/search-locations-${test.info().project.name}.png`,
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
