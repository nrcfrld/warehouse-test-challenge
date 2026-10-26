import { loadEnv } from "vite";

export function proxyTarget(mode: string, envDir: string) {
  const env = loadEnv(mode, envDir, "API_PROXY_TARGET");
  return (
    process.env.API_PROXY_TARGET ||
    env.API_PROXY_TARGET ||
    "http://localhost:8080"
  );
}
