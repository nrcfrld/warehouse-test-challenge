import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { spawnSync } from "node:child_process";
const base = process.env.E2E_BASE_URL || "http://localhost:8088";
async function request(path, init) {
  const response = await fetch(`${base}${path}`, init);
  const body = await response.json();
  assert.equal(response.ok, true, `${path}: ${JSON.stringify(body)}`);
  return body;
}
const body = await request("/api/v1/items", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    sku: `SMOKE-${randomUUID()}`,
    name: "Persistence smoke test",
    category: "Verification",
    unit: "kg",
  }),
});
const id = body.data.id;
try {
  const restart = spawnSync("docker", ["compose", "restart", "db", "backend"], {
    stdio: "inherit",
  });
  assert.equal(restart.status, 0);
  let ready = false;
  let lastProbe = "No readiness response";
  for (let i = 0; i < 30; i++) {
    try {
      const response = await fetch(`${base}/health/ready`, {
        signal: AbortSignal.timeout(2000),
      });
      lastProbe = `HTTP ${response.status}: ${await response.text()}`;
      if (response.ok) {
        ready = true;
        break;
      }
    } catch (error) {
      lastProbe = error.message;
      /* transient restart connection */
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  assert.equal(
    ready,
    true,
    `Application did not recover through Nginx after restart. Last probe: ${lastProbe}`,
  );
  assert.equal((await request(`/api/v1/items/${id}`)).data.id, id);
  // A bad database target must fail migration rather than report success.
  const invalid = spawnSync(
    "docker",
    [
      "compose",
      "run",
      "--rm",
      "--no-deps",
      "-e",
      "DATABASE_URL=postgres://warehouse:warehouse_local@db:5432/warehouse_missing_smoke?sslmode=disable",
      "migrate",
    ],
    { encoding: "utf8" },
  );
  assert.notEqual(
    invalid.status,
    0,
    "Migration unexpectedly accepted a missing database",
  );
  assert.match(
    invalid.stderr,
    /does not exist/,
    "Expected an explicit missing-database migration error",
  );
  console.log(
    "PASS: readiness, persistent data across restart, migration failure",
  );
} finally {
  await request(`/api/v1/items/${id}`, { method: "DELETE" }).catch((error) =>
    console.error("Cleanup failed:", error.message),
  );
}
