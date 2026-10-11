const CACHE_NAME = 'cordys-crm-pwa-v2'
const APP_SHELL = ['/', '/manifest.webmanifest', '/icons/app-icon.svg']
const IS_DEV = ['localhost', '127.0.0.1'].includes(self.location.hostname) && self.location.port === '5173'

self.addEventListener('install', event => {
  if (IS_DEV) { self.skipWaiting(); return }
  event.waitUntil(caches.open(CACHE_NAME).then(cache => cache.addAll(APP_SHELL)))
  self.skipWaiting()
})

self.addEventListener('activate', event => {
  if (IS_DEV) {
    event.waitUntil(self.registration.unregister().then(() => caches.delete(CACHE_NAME)))
    return
  }
  event.waitUntil(caches.keys().then(keys => Promise.all(keys.filter(key => key !== CACHE_NAME).map(key => caches.delete(key)))))
  self.clients.claim()
})

self.addEventListener('fetch', event => {
  if (IS_DEV) return
  const request = event.request
  if (request.method !== 'GET' || new URL(request.url).origin !== self.location.origin || new URL(request.url).pathname.startsWith('/api/')) return
  if (request.mode === 'navigate') {
    event.respondWith(fetch(request).then(response => {
      const copy = response.clone()
      caches.open(CACHE_NAME).then(cache => cache.put('/', copy))
      return response
    }).catch(() => caches.match('/')))
    return
  }
  event.respondWith(caches.match(request).then(cached => cached || fetch(request).then(response => {
    if (response.ok) caches.open(CACHE_NAME).then(cache => cache.put(request, response.clone()))
    return response
  })))
})
