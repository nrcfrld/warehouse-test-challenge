import { StrictMode, type CSSProperties } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { APIError } from "./services/api";
import { TooltipProvider } from "./components/ui/tooltip";
import App from "./app";
import "./index.css";
const client = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (count, error) =>
        count < 1 && (!(error instanceof APIError) || error.status >= 500),
      refetchOnWindowFocus: false,
    },
    mutations: { retry: false },
  },
});
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={client}>
      <TooltipProvider delayDuration={250}>
        <App />
        <Toaster
          richColors
          expand
          theme="light"
          closeButton
          position="bottom-right"
          style={
            {
              "--success-text": "#116329",
              "--error-text": "#a31515",
            } as CSSProperties
          }
        />
      </TooltipProvider>
    </QueryClientProvider>
  </StrictMode>,
);
