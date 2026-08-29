// 极简 app-shell SW：静态资产 cache-first（带版本号清缓存），页面/接口 network-first。
// 不缓存 /api 与 /mcp 响应——数据永远以服务端为准。
const VERSION = "fund-v1";
const STATIC_CACHE = `${VERSION}-static`;

self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => keys.filter((k) => !k.startsWith(VERSION)).map((k) => caches.delete(k)))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/mcp")) return;

  // 带 hash 的构建资产：cache-first（immutable）
  if (url.pathname.startsWith("/assets/")) {
    event.respondWith(
      caches.match(event.request).then(
        (hit) =>
          hit ??
          fetch(event.request).then((res) => {
            if (res.ok) {
              const clone = res.clone();
              caches.open(STATIC_CACHE).then((c) => c.put(event.request, clone));
            }
            return res;
          }),
      ),
    );
    return;
  }

  // HTML 与其余：network-first，离线回退到缓存的导航壳
  if (event.request.mode === "navigate") {
    event.respondWith(
      fetch(event.request)
        .then((res) => {
          if (res.ok) {
            const clone = res.clone();
            caches.open(STATIC_CACHE).then((c) => c.put("/", clone));
          }
          return res;
        })
        .catch(() => caches.match("/").then((hit) => hit ?? Response.error())),
    );
  }
});
