// Regen PWA service worker
// CACHE_NAME: bump this value on every deploy to evict stale shells.
const CACHE_NAME = 'regen-shell-v1'

// __SHELL_ASSETS__ — replaced by scripts/inject-sw-assets.mjs at build time.
// In development (served directly from public/) this serves only the root.
const SHELL_ASSETS = ['/']

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL_ASSETS))
  )
  // Take control immediately — don't wait for the old SW to finish.
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  // Delete all caches that don't match the current CACHE_NAME.
  // This evicts shells from previous deploys.
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
    )
  )
  self.clients.claim()
})

self.addEventListener('fetch', (event) => {
  // Never intercept non-GET requests or API calls.
  if (event.request.method !== 'GET') return
  if (new URL(event.request.url).pathname.startsWith('/api/')) return

  event.respondWith(
    caches.match(event.request).then((cached) => cached || fetch(event.request))
  )
})

self.addEventListener('push', (event) => {
  let payload = { title: 'Regen', body: 'New notification', data: {} }
  if (event.data) {
    try {
      payload = event.data.json()
    } catch (_) {
      // malformed payload — use defaults
    }
  }
  event.waitUntil(
    self.registration.showNotification(payload.title, {
      body: payload.body,
      icon: '/icon-192.png',
      badge: '/icon-192.png',
      data: payload.data || {},
    })
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const incidentId = event.notification.data && event.notification.data.incident_id
  const url = incidentId ? `/incidents/${incidentId}` : '/'

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      // Focus an existing window if one is open.
      for (const client of windowClients) {
        if ('focus' in client) {
          client.focus()
          return
        }
      }
      // Otherwise open a new window.
      if (clients.openWindow) {
        return clients.openWindow(url)
      }
    })
  )
})
