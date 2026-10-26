import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
afterEach(() => {
  cleanup();
  sessionStorage.clear();
});
// JSDOM has no layout engine; Radix observes element sizes in real browsers.
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// cmdk scrolls the active option into view; JSDOM has no scrolling layout.
HTMLElement.prototype.scrollIntoView = function () {};
