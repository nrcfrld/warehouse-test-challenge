import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
const base = process.env.E2E_BASE_URL || "http://localhost:8088";
function compose(args) {
  const result = spawnSync("docker", ["compose", ...args], {
    encoding: "utf8",
  });
  assert.equal(result.status, 0, result.stderr);
  return result.stdout.trim();
}
function redis(...args) {
  return compose(["exec", "-T", "redis", "redis-cli", "--raw", ...args]);
}
async function request(path, init) {
  const response = await fetch(`${base}${path}`, init);
  const body = await response.json();
  assert.equal(response.ok, true, `${path}: ${JSON.stringify(body)}`);
  return body.data;
}
const category = `Cache smoke ${randomUUID()}`;
let id;
try {
  await request("/api/v1/locations");
  await request("/api/v1/categories");
  assert.ok(
    JSON.parse(redis("HGET", "warehouse:v1:lookups:locations", "value"))
      .length > 0,
  );
  assert.ok(redis("PTTL", "warehouse:v1:lookups:locations") > 0);
  assert.ok(
    !JSON.parse(
      redis("HGET", "warehouse:v1:lookups:categories", "value"),
    ).includes(category),
  );
  const item = await request("/api/v1/items", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      sku: randomUUID(),
      name: "Cache smoke test",
      category,
      unit: "pcs",
    }),
  });
  id = item.id;
  assert.equal(
    redis("EXISTS", "warehouse:v1:lookups:categories"),
    "0",
    "Item creation must invalidate categories",
  );
  assert.ok((await request("/api/v1/categories")).includes(category));
  assert.ok(
    JSON.parse(
      redis("HGET", "warehouse:v1:lookups:categories", "value"),
    ).includes(category),
  );
  await request(`/api/v1/items/${id}`, { method: "DELETE" });
  id = undefined;
  assert.ok(!(await request("/api/v1/categories")).includes(category));
  // Stop only disposable cache storage. API readiness and lookups must still work.
  compose(["stop", "redis"]);
  await request("/health/ready");
  const started = Date.now();
  assert.ok((await request("/api/v1/locations")).length > 0);
  assert.ok(Array.isArray(await request("/api/v1/categories")));
  assert.ok(
    Date.now() - started < 2000,
    "Redis outage exceeded the fallback budget",
  );
} finally {
  compose(["up", "-d", "--no-deps", "--wait", "redis"]);
  if (id) await request(`/api/v1/items/${id}`, { method: "DELETE" });
}
await request("/api/v1/locations");
assert.ok(
  JSON.parse(redis("HGET", "warehouse:v1:lookups:locations", "value")).length >
    0,
);
console.log(
  "PASS: Redis cache fill, TTL, invalidation, outage fallback, and recovery",
);
