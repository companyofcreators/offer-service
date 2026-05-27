package ws

import (
	"crypto/rsa"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	wsinfra "github.com/companyofcreators/offer-service/internal/infrastructure/ws"
)

// Handler manages WebSocket connections for offer subscriptions.
type Handler struct {
	hub           *wsinfra.Hub
	jwtPublicKey  *rsa.PublicKey
	log           *slog.Logger
	allowedOrigin string
	upgrader      websocket.Upgrader
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *wsinfra.Hub, jwtPublicKey *rsa.PublicKey, log *slog.Logger, allowedOrigin string) *Handler {
	h := &Handler{
		hub:           hub,
		jwtPublicKey:  jwtPublicKey,
		log:           log,
		allowedOrigin: allowedOrigin,
	}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	return h
}

func (h *Handler) checkOrigin(r *http.Request) bool {
	if h.allowedOrigin != "" {
		origin := r.Header.Get("Origin")
		return origin == h.allowedOrigin
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == "http://"+r.Host || origin == "https://"+r.Host {
		return true
	}
	originLower := strings.ToLower(origin)
	return strings.Contains(originLower, "//localhost:") ||
		strings.Contains(originLower, "//127.0.0.1:") ||
		strings.Contains(originLower, "//192.168.0.103")
}

// Upgrade handles HTTP upgrade to WebSocket.
// Requires ?token=JWT and ?order_id=UUID query parameters.
func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "отсутствует параметр token", http.StatusUnauthorized)
		return
	}

	orderIDStr := r.URL.Query().Get("order_id")
	if orderIDStr == "" {
		http.Error(w, "отсутствует параметр order_id", http.StatusBadRequest)
		return
	}

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		http.Error(w, "недействительный order_id", http.StatusBadRequest)
		return
	}

	userID, err := h.validateToken(tokenStr)
	if err != nil {
		h.log.Warn("invalid JWT token for ws", "error", err)
		http.Error(w, "недействительный токен", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("failed to upgrade ws connection", "error", err)
		return
	}

	h.hub.RegisterClient(userID, orderID, conn)
	h.log.Info("ws client registered for offers",
		"user_id", userID.String(),
		"order_id", orderID.String(),
	)
}

func (h *Handler) validateToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("неожиданный метод подписи")
		}
		return h.jwtPublicKey, nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("недействительные claims")
	}

	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, errors.New("отсутствует sub claim")
	}

	return uuid.Parse(userIDStr)
}
