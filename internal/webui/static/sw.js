/* DeckTap Service Worker – PWA offline caching & installability */
const CACHE_NAME = "decktap-v2.3";
const ASSETS = [
  "./",
  "index.html",
  "styles.css",
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

  e.respondWith(
    fetch(e.request).catch(() => caches.match(e.request))
  );
});
