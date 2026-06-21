import '@testing-library/jest-dom';

// Configure i18next for tests — use Chinese so existing test expectations match
import i18n from '../i18n';
i18n.changeLanguage('zh');

// jsdom does not implement ResizeObserver, which is used by echarts-dependent components
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
} as any;

// jsdom does not implement EventSource (used by useSSE hook)
globalThis.EventSource = class EventSource {
  url: string;
  onmessage: ((e: any) => void) | null = null;
  onerror: ((e: any) => void) | null = null;
  onopen: ((e: any) => void) | null = null;
  readyState = 0;
  _listeners: Record<string, Array<(e: any) => void>> = {};
  constructor(url: string) { this.url = url; }
  addEventListener(type: string, fn: (e: any) => void) {
    if (!this._listeners[type]) this._listeners[type] = [];
    this._listeners[type].push(fn);
  }
  removeEventListener(type: string, fn: (e: any) => void) {
    if (this._listeners[type]) this._listeners[type] = this._listeners[type].filter(f => f !== fn);
  }
  close() { this.readyState = 2; }
} as any;
