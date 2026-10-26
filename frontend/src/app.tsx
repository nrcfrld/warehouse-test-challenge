import { Component, type ErrorInfo, type ReactNode } from "react";
import {
  createBrowserRouter,
  Link,
  NavLink,
  Outlet,
  RouterProvider,
  useLocation,
  useRouteError,
  Navigate,
} from "react-router-dom";
import {
  BookOpen,
  MapPin,
  Box,
  ChevronRight,
  Layers,
  Package,
  ArrowUpRight,
} from "lucide-react";
import { Loading } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Tooltip } from "@/components/ui/tooltip";
function RouteError() {
  const error = useRouteError();
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-5 p-8">
      <h1 className="text-2xl font-semibold">This page could not be opened</h1>
      <p className="text-muted-foreground">
        {error instanceof Error
          ? "Please reload the page to try again."
          : "Check the address or return to your inventory."}
      </p>
      <Button asChild>
        <a href="/items">Return to inventory</a>
      </Button>
    </div>
  );
}
class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Application render failed", error, info.componentStack);
  }
  render() {
    return this.state.failed ? (
      <div className="page">
        <h1 className="text-2xl font-semibold">Something went wrong</h1>
        <Button className="mt-5" onClick={() => window.location.reload()}>
          Reload application
        </Button>
      </div>
    ) : (
      this.props.children
    );
  }
}
function Layout() {
  const { pathname } = useLocation();
  const isLocations = pathname.startsWith("/locations");
  const isList = pathname === "/items" || isLocations;
  const label = pathname.endsWith("/new")
    ? "New item"
    : pathname.endsWith("/edit")
      ? "Edit item"
      : "Item details";
  return (
    <div className="min-h-screen">
      <a
        href="#main"
        className="sr-only z-50 focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:rounded-lg focus:bg-white focus:p-3"
      >
        Skip to content
      </a>
      <aside
        aria-label="Main navigation"
        className="fixed inset-y-0 left-0 z-20 flex w-[76px] flex-col border-r bg-[#fafbfc] lg:w-[220px]"
      >
        <Link
          to="/items"
          className="mx-auto mt-7 flex items-center gap-3 lg:mx-6"
          aria-label="Warehouse home"
        >
          <span className="flex size-10 shrink-0 items-center justify-center rounded-[13px] bg-primary text-white shadow-[0_3px_8px_#0069df20]">
            <Box className="size-[23px]" strokeWidth={1.7} />
          </span>
          <span className="hidden text-[17px] font-semibold tracking-tight lg:block">
            Warehouse
            <span className="mt-0.5 block text-[10px] font-normal tracking-wide text-muted-foreground">
              A PLACE FOR EVERYTHING
            </span>
          </span>
        </Link>
        <div className="mt-12 px-3 lg:px-4">
          <p className="eyebrow mb-3 hidden px-3 lg:block">Workspace</p>
          <Tooltip content="Inventory">
            <NavLink
              to="/items"
              className={`flex h-11 items-center justify-center gap-3 rounded-xl text-sm font-medium lg:justify-start lg:px-3 ${!isLocations ? "bg-blue-50 text-primary" : "text-muted-foreground hover:bg-muted"}`}
              aria-label="Inventory"
            >
              <Layers className="size-[18px]" />
              <span className="hidden lg:inline">Inventory</span>
            </NavLink>
          </Tooltip>
          <Tooltip content="Locations">
            <NavLink
              to="/locations"
              aria-label="Locations"
              className={`mt-2 flex h-11 items-center justify-center gap-3 rounded-xl text-sm font-medium lg:justify-start lg:px-3 ${isLocations ? "bg-blue-50 text-primary" : "text-muted-foreground hover:bg-muted"}`}
            >
              <MapPin className="size-[18px]" />
              <span className="hidden lg:inline">Locations</span>
            </NavLink>
          </Tooltip>
        </div>
        <div className="mt-auto px-3 pb-5 lg:px-4">
          <Tooltip content="API documentation">
            <a
              href="/docs"
              target="_blank"
              rel="noreferrer"
              className="flex h-11 items-center justify-center gap-3 rounded-xl text-sm text-muted-foreground hover:bg-muted lg:justify-start lg:px-3"
              aria-label="API documentation, opens in a new tab"
            >
              <BookOpen className="size-[18px]" />
              <span className="hidden lg:inline">API docs</span>
              <ArrowUpRight className="ml-auto hidden size-3.5 lg:block" />
            </a>
          </Tooltip>
          <div className="mx-2 mt-5 hidden border-t pt-5 lg:block">
            <p className="text-xs font-medium">Local workspace</p>
            <p className="mt-1 text-[11px] text-muted-foreground">
              Warehouse management · v1.0
            </p>
          </div>
        </div>
      </aside>
      <div className="ml-[76px] lg:ml-[220px]">
        <header className="flex h-18 items-center justify-between border-b bg-white/80 px-6 lg:px-12">
          <nav
            aria-label="Breadcrumb"
            className="flex items-center gap-3 text-xs"
          >
            <Package
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
            <Link
              to={isLocations ? "/locations" : "/items"}
              className={
                isList
                  ? "font-medium"
                  : "text-muted-foreground hover:text-primary"
              }
            >
              {isLocations ? "Locations" : "Inventory"}
            </Link>
            {!isList && (
              <>
                <ChevronRight
                  className="size-3 text-muted-foreground"
                  aria-hidden="true"
                />
                <span className="font-medium">{label}</span>
              </>
            )}
          </nav>
          <span className="text-[11px] tracking-wide text-muted-foreground">
            WAREHOUSE / 01
          </span>
        </header>
        <main id="main" tabIndex={-1}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
const router = createBrowserRouter([
  {
    element: <Layout />,
    errorElement: <RouteError />,
    children: [
      {
        path: "/locations",
        lazy: async () => ({
          Component: (await import("@/pages/locations")).default,
        }),
      },
      { path: "/", element: <Navigate to="/items" replace /> },
      {
        path: "/items",
        lazy: async () => ({
          Component: (await import("@/pages/items")).default,
        }),
      },
      {
        path: "/items/new",
        lazy: async () => ({
          Component: (await import("@/pages/item-form")).default,
        }),
      },
      {
        path: "/items/:id",
        lazy: async () => ({
          Component: (await import("@/pages/item-detail")).default,
        }),
      },
      {
        path: "/items/:id/edit",
        lazy: async () => ({
          Component: (await import("@/pages/item-form")).default,
        }),
      },
    ],
  },
]);
export default function App() {
  return (
    <ErrorBoundary>
      <RouterProvider
        router={router}
        fallbackElement={<Loading rows={4} />}
        future={{ v7_startTransition: true }}
      />
    </ErrorBoundary>
  );
}
