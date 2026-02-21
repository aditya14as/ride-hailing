package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// NewRelicMiddleware creates a middleware that instruments requests with New Relic
func NewRelicMiddleware(app *newrelic.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if app == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get the route pattern for better transaction naming
			routePattern := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					routePattern = pattern
				}
			}

			txnName := r.Method + " " + routePattern
			txn := app.StartTransaction(txnName)
			defer txn.End()

			txn.SetWebRequestHTTP(r)
			w = txn.SetWebResponse(w)

			// Add transaction to context
			r = newrelic.RequestWithTransactionContext(r, txn)

			next.ServeHTTP(w, r)
		})
	}
}
