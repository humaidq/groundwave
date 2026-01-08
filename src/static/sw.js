const CACHE_NAME = 'groundwave-v3';
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
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))
    )
  );
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;

  event.respondWith(
    caches.match(event.request).then((cached) =>
      fetch(event.request)
        .then((response) => {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
          return response;
        })
        .catch(() => {
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
