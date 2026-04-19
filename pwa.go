package main

import "net/http"

func manifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	_, _ = w.Write([]byte(`{
  "name": "LAN Quiz",
  "short_name": "Quiz",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#0b1020",
  "theme_color": "#0b1020",
  "icons": []
}`))
}

func serviceWorkerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte(`
const CACHE_NAME = 'lan-quiz-v2';
const URLS = ['/', '/host', '/screen', '/manifest.webmanifest'];

self.addEventListener('install', event => {
  event.waitUntil(caches.open(CACHE_NAME).then(cache => cache.addAll(URLS)));
});

self.addEventListener('fetch', event => {
  if (event.request.method !== 'GET') return;

  // Для HTML-страниц всегда пробуем сеть первой,
  // чтобы не показывать устаревшую версию после запуска.
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(() => caches.match(event.request))
    );
    return;
  }

  // Для остальных GET оставляем cache-first.
  event.respondWith(caches.match(event.request).then(resp => resp || fetch(event.request)));
});
`))
}
