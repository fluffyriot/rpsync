const CACHE_NAME = 'rpsync-v3';
const ASSETS = [
    '/',
    '/static/css/index.css',
    '/static/manifest.json',
    '/static/images/logo.svg',
    '/static/images/favicon-32.png',
    '/static/images/pwa/pwa_icon_x192.png',
    '/static/images/pwa/pwa_icon_x512.png',
    'https://unpkg.com/lucide@latest/dist/umd/lucide.js',
    'https://cdn.jsdelivr.net/npm/chart.js'
];

self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then((cache) => cache.addAll(ASSETS))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((cacheNames) => {
            return Promise.all(
                cacheNames.map((cacheName) => {
                    if (cacheName !== CACHE_NAME) {
                        return caches.delete(cacheName);
                    }
                })
            );
        }).then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', (event) => {
    event.respondWith(
        caches.match(event.request)
            .then((response) => {
                if (response) {
                    return response;
                }
                return fetch(event.request);
            })
    );
});
