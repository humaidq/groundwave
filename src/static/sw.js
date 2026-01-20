const CACHE_NAME = 'groundwave-v4';
const STATIC_ASSETS = [
  '/main.css',
  '/normalize-8.0.1.min.css',
  '/icon.svg',
  '/icon-64.png',
  '/icon-128.png',
  '/icon-512.png'
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  // Force the waiting service worker to become active immediately
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  // IMPORTANT: Only intercept GET requests
  // POST, PUT, DELETE, etc. must pass through without interference
  // to allow forms and CSRF tokens to work correctly
  if (event.request.method !== 'GET') {
    // Do not intercept - let the request pass through to the network
    return;
  }

  const url = new URL(event.request.url);
  if (url.pathname === '/connectivity') {
    return;
  }

  // Only cache and serve static assets and pages
  event.respondWith(
    caches.match(event.request).then((cached) =>
      fetch(event.request)
        .then((response) => {
          // Only cache successful responses
          if (response.status === 200) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
          }
          return response;
        })
        .catch(() => {
          // Network failed, try to serve from cache
          if (cached) return cached;

          // Only for uncached navigation requests, return minimal offline page
          if (event.request.mode === 'navigate') {
            return new Response('<!DOCTYPE html><html><body><p>Page not available offline</p></body></html>', {
              status: 503,
              headers: { 'Content-Type': 'text/html' }
            });
          }
          return new Response('', { status: 503 });
        })
    )
  );
});
