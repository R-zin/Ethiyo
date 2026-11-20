/* Ethiyo service worker: installable app + offline shell.
 *
 * Live data is never cached. /api/, /health and /metrics (including the SSE
 * streams) always go straight to the network, so the app can never show a
 * cached position pretending to be live. Only the app shell and the hashed,
 * immutable build assets are cached.
 */
const VERSION = 'ethiyo-shell-v1'
const SHELL = ['/', '/manifest.webmanifest', '/bus.svg', '/icons/icon-192.png', '/icons/icon-512.png']
const LIVE_PREFIXES = ['/api/', '/health', '/metrics']

/** Decides how a request is served: 'network' (not intercepted), 'network-first', 'cache-first' or 'stale-while-revalidate'. */
function strategyFor(request, origin) {
  const url = new URL(request.url)
  if (request.method !== 'GET') return 'network'
  if (url.origin !== origin) return 'network' // map tiles, fonts: leave to the HTTP cache
  if (LIVE_PREFIXES.some(prefix => url.pathname.startsWith(prefix))) return 'network'
  if (request.mode === 'navigate') return 'network-first'
  if (url.pathname.startsWith('/assets/')) return 'cache-first' // content-hashed by Vite
  return 'stale-while-revalidate'
}

async function networkFirst(request) {
  const cache = await caches.open(VERSION)
  try {
    const response = await fetch(request)
    if (response.ok) await cache.put('/', response.clone()) // every route renders the same shell
    return response
  } catch {
    return (await cache.match('/')) || Response.error()
  }
}

async function cacheFirst(request) {
  const cache = await caches.open(VERSION)
  const cached = await cache.match(request)
  if (cached) return cached
  const response = await fetch(request)
  if (response.ok) await cache.put(request, response.clone())
  return response
}

async function staleWhileRevalidate(request, event) {
  const cache = await caches.open(VERSION)
  const cached = await cache.match(request)
  const refresh = fetch(request)
    .then(response => {
      if (response.ok) return cache.put(request, response.clone()).then(() => response)
      return response
    })
    .catch(() => cached || Response.error())
  if (cached) {
    event.waitUntil(refresh)
    return cached
  }
  return refresh
}

self.addEventListener('install', event => {
  event.waitUntil(
    caches
      .open(VERSION)
      .then(cache => cache.addAll(SHELL))
      .then(() => self.skipWaiting())
  )
})

self.addEventListener('activate', event => {
  event.waitUntil(
    caches
      .keys()
      .then(keys => Promise.all(keys.filter(key => key !== VERSION).map(key => caches.delete(key))))
      .then(() => self.clients.claim())
  )
})

self.addEventListener('fetch', event => {
  const strategy = strategyFor(event.request, self.location.origin)
  if (strategy === 'network') return // not intercepted: the browser fetches it normally
  if (strategy === 'network-first') event.respondWith(networkFirst(event.request))
  else if (strategy === 'cache-first') event.respondWith(cacheFirst(event.request))
  else event.respondWith(staleWhileRevalidate(event.request, event))
})

// Tapping an alert notification focuses the app (or opens it).
self.addEventListener('notificationclick', event => {
  event.notification.close()
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(windows => {
      const existing = windows.find(w => 'focus' in w)
      return existing ? existing.focus() : self.clients.openWindow('/')
    })
  )
})
