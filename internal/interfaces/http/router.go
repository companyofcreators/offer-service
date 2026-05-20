package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(handler *Handler, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(requestLoggerMiddleware(log))

	r.Route("/internal", func(r chi.Router) {
		// Health check
		r.Get("/health", handler.Health)

		// Offer management
		r.Post("/offers", handler.SendOffer)
		r.Get("/offers", handler.ListOffers)
		r.Get("/offers/{id}", handler.GetOffer)
		r.Get("/offers/{id}/history", handler.GetOfferHistory)

		// Offer actions
		r.Post("/offers/{id}/withdraw", handler.WithdrawOffer)
		r.Post("/offers/{id}/accept", handler.AcceptOffer)
		r.Post("/offers/{id}/reject", handler.RejectOffer)
		r.Post("/offers/{id}/counter", handler.CounterOffer)

		// Order negotiation history
		r.Get("/orders/{id}/history", handler.GetOrderHistory)
	})

	return r
}

func requestLoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.InfoContext(r.Context(), "request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
			)
		})
	}
}
