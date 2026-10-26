import { spawnSync } from "node:child_process";
const go = spawnSync("go", ["version"], { encoding: "utf8" });
const result = spawnSync(
  "npx",
  ["--no-install", "turbo", "run", ...process.argv.slice(2)],
  {
    stdio: "inherit",
    env: {
      ...process.env,
      TOOLCHAIN_CACHE_KEY: `${process.platform}/${process.arch}/${process.version}/${go.stdout?.trim()}`,
    },
  },
);
process.exit(result.status ?? 1);
