package pep

import "net/http"

// CORS wraps h with cross-origin handling for the configured allowlist. It
// exists for the reviewer browser extension: a content-script fetch carries the
// page origin and is subject to CORS, so the extension's origin
// (e.g. "chrome-extension://<id>") must be allowed. Extensions that fetch from a
// background service worker with host_permissions bypass CORS entirely, so an
// empty allowlist (the default) simply disables this and is the common case.
// See docs/auth-design.md §4.1.1.
//
// Only origins in allowed get CORS headers; anything else passes through
// unchanged (the browser then blocks it, as intended). Credentials are not
// enabled because the extension authenticates with a Bearer token, not cookies.
func CORS(allowed []string, h http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		_, ok := set[origin]
		if ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		// Answer the preflight here regardless of match: a disallowed origin
		// gets a 204 without the allow headers, which the browser treats as a
		// CORS failure (the intended outcome).
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
