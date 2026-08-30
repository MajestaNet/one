// Monaco is aliased to src/test-stubs/monaco-editor.ts in vitest.config.ts.

/** jsdom does not implement matchMedia; Vitest 4+ no longer polyfills it. */
function matchMediaPolyfill(query: string): MediaQueryList {
  return {
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  } as MediaQueryList;
}

if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  window.matchMedia = matchMediaPolyfill;
}

/** assistant-ui ThreadPrimitive.Viewport uses ResizeObserver (not in jsdom). */
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver;
}

/** assistant-ui auto-scroll calls element.scrollTo (missing in jsdom). */
function scrollToPolyfill(this: Element, ..._args: unknown[]) {
  /* no-op in jsdom */
}

if (typeof Element !== "undefined") {
  Element.prototype.scrollTo = scrollToPolyfill as typeof Element.prototype.scrollTo;
}
if (typeof HTMLElement !== "undefined") {
  HTMLElement.prototype.scrollTo = scrollToPolyfill as typeof HTMLElement.prototype.scrollTo;
}
