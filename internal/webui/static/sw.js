/* DeckTap Service Worker – PWA offline caching, resilient recovery & installability */
const CACHE_NAME = "decktap-v2.10";
const ASSETS = [
  "./",
  "index.html",
  "styles.css",
  "jsqr.js",
  "app.js",
  "crypto.js",
  "icon.svg",
  "manifest.webmanifest"
];

self.addEventListener("install", (e) => {
  e.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(ASSETS)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.map((k) => (k !== CACHE_NAME ? caches.delete(k) : null)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (e) => {
  // Never intercept WebSocket connections
  if (e.request.url.startsWith("ws:") || e.request.url.startsWith("wss:")) return;
  if (e.request.method !== "GET") return;

  e.respondWith(
    fetch(e.request)
      .then((res) => {
        // If server returns HTTP 5xx (e.g. Cloudflare Error 1033 / 530 / 502), recover with cached asset
        if (!res.ok && res.status >= 500) {
          return caches.match(e.request).then((cached) => {
            if (cached) return cached;
            if (e.request.mode === "navigate") {
              return caches.match("index.html").then((idx) => idx || res);
            }
            return res;
          });
        }
        return res;
      })
      .catch(() => {
        return caches.match(e.request).then((cached) => {
          if (cached) return cached;
          if (e.request.mode === "navigate") {
            return caches.match("index.html");
          }
          return new Response("Offline", { status: 503, statusText: "Offline" });
        });
      })
  );
});
