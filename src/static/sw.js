const CACHE_NAME = 'groundwave-v5';
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

  if (url.origin === self.location.origin && STATIC_ASSETS.includes(url.pathname)) {
    event.respondWith(
      caches.match(event.request).then((cached) => {
        const fetchPromise = fetch(event.request, { cache: 'no-store' })
          .then((response) => {
            if (response.ok) {
              const responseClone = response.clone();
              event.waitUntil(
                caches.open(CACHE_NAME).then((cache) => cache.put(event.request, responseClone))
              );
            }
            return response;
          })
          .catch(() => cached || new Response('', { status: 503 }));

        if (cached) {
          event.waitUntil(fetchPromise.catch(() => undefined));
          return cached;
        }

        return fetchPromise;
      })
    );
    return;
  }

  // Do not cache dynamic or HTML responses for security reasons.
  // Non-static requests pass through to the network unchanged.
});
