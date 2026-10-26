import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyTarget } from "../../vite-proxy";

let directory: string;
beforeEach(() => {
  directory = mkdtempSync(join(tmpdir(), "warehouse-proxy-"));
  vi.stubEnv("API_PROXY_TARGET", undefined);
});
afterEach(() => {
  rmSync(directory, { recursive: true, force: true });
  vi.unstubAllEnvs();
});
it("loads the proxy target from .env.local", () => {
  writeFileSync(
    join(directory, ".env.local"),
    "API_PROXY_TARGET=http://localhost:9099\n",
  );
  expect(proxyTarget("development", directory)).toBe("http://localhost:9099");
});
it("gives the shell environment precedence over env files", () => {
  writeFileSync(
    join(directory, ".env.local"),
    "API_PROXY_TARGET=http://localhost:9099\n",
  );
  vi.stubEnv("API_PROXY_TARGET", "http://localhost:9098");
  expect(proxyTarget("development", directory)).toBe("http://localhost:9098");
});
it("uses the local API when no override is configured", () => {
  expect(proxyTarget("development", directory)).toBe("http://localhost:8080");
});
