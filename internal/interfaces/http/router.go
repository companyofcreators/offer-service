package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/companyofcreators/offer-service/pkg/header_auth"
)

// WebSocketUpgrader is the function type for upgrading HTTP connections to WebSocket.
type WebSocketUpgrader func(w http.ResponseWriter, r *http.Request)

// NewRouter creates and configures the HTTP router.
func NewRouter(handler *Handler, signer *header_auth.HeaderSigner, log *slog.Logger, wsUpgrader WebSocketUpgrader) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applies to all routes including WebSocket)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(bodySizeLimiter(500 << 10)) // 500KB
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(requestLoggerMiddleware(log))

	// WebSocket endpoint — JWT validation is done inside the handler,
	// no HMAC header verification needed.
	r.Get("/ws", http.HandlerFunc(wsUpgrader))

	// Internal routes — protected by HMAC header verification.
	r.Route("/internal", func(r chi.Router) {
		r.Use(signer.VerifyMiddleware)

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

// bodySizeLimiter returns middleware that wraps http.MaxBytesReader to limit
// request body size and prevent memory exhaustion attacks.
func bodySizeLimiter(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
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
