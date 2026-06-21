/** Simple in-memory TTL cache for external API calls */

interface CacheEntry<T> {
  data: T;
  expires: number;
}

const store = new Map<string, CacheEntry<unknown>>();
let hits = 0;
let misses = 0;
let maxSize = 500;

/** Configure the maximum number of cache entries. */
export function setCacheMaxSize(n: number): void {
  maxSize = n > 0 ? n : 500;
  while (store.size > maxSize) {
    evictOldest();
  }
}

function evictOldest(): void {
  const oldest = store.keys().next().value;
  if (oldest !== undefined) store.delete(oldest);
}

/** Get or compute a cached value. */
export async function cachedFetch<T>(key: string, ttlMs: number, fetcher: () => Promise<T>): Promise<T> {
  const entry = store.get(key);
  const now = Date.now();
  if (entry && entry.expires > now) {
    hits++;
    // Move entry to the end of the Map so it stays "young" (LRU, not FIFO).
    store.delete(key);
    store.set(key, entry);
    return entry.data as T;
  }
  misses++;
  const data = await fetcher();
  // LRU eviction: if the store is full and this is a new key, drop the oldest.
  if (!store.has(key) && store.size >= maxSize) {
    evictOldest();
  }
  // Delete first so re-insertion puts the key at the youngest position.
  store.delete(key);
  store.set(key, { data, expires: now + ttlMs });
  return data;
}

/** Clear all cache entries. */
export function clearCache(): void {
  store.clear();
  hits = 0;
  misses = 0;
}

/** Get cache statistics. */
export function getCacheStats(): { size: number; hits: number; misses: number; hitRate: number } {
  const total = hits + misses;
  return {
    size: store.size,
    hits,
    misses,
    hitRate: total > 0 ? Math.round((hits / total) * 100) / 100 : 0,
  };
}

// ── Default TTLs ───────────────────────────────────────────────────────

export const TTL = {
  STOCK_QUOTE: 5 * 60 * 1000,       // 5 minutes
  NAV_HISTORY: 30 * 60 * 1000,      // 30 minutes
  FUND_DETAILS: 60 * 60 * 1000,     // 1 hour
  INDEX_QUOTE: 5 * 60 * 1000,       // 5 minutes
  FUND_HOLDINGS: 24 * 60 * 60 * 1000, // 24 hours (quarterly data)
} as const;
