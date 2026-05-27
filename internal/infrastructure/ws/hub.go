package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a single WebSocket connection subscribed to offers for a specific order.
type Client struct {
	UserID  uuid.UUID
	OrderID uuid.UUID
	conn    *websocket.Conn
	hub     *Hub
	send    chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains clients grouped by order_id and broadcasts offer events.
type Hub struct {
	// clients[orderID][client] = struct{}
	clients    map[uuid.UUID]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	log        *slog.Logger
}

// OfferEvent is the JSON payload sent over WebSocket.
type OfferEvent struct {
	Type  string          `json:"type"`
	Offer json.RawMessage `json:"offer,omitempty"`
}

// NewHub creates a new Hub and starts its run loop.
func NewHub(log *slog.Logger) *Hub {
	h := &Hub{
		clients:    make(map[uuid.UUID]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		log:        log,
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.OrderID]; !ok {
				h.clients[client.OrderID] = make(map[*Client]struct{})
			}
			h.clients[client.OrderID][client] = struct{}{}
			h.mu.Unlock()
			h.log.Info("ws client subscribed to offers",
				"user_id", client.UserID.String(),
				"order_id", client.OrderID.String(),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.OrderID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.OrderID)
					}
				}
			}
			h.mu.Unlock()
			h.log.Info("ws client unsubscribed from offers",
				"user_id", client.UserID.String(),
				"order_id", client.OrderID.String(),
			)
		}
	}
}

// RegisterClient registers a new WebSocket client for a given order.
func (h *Hub) RegisterClient(userID, orderID uuid.UUID, conn *websocket.Conn) *Client {
	client := &Client{
		UserID:  userID,
		OrderID: orderID,
		conn:    conn,
		hub:     h,
		send:    make(chan []byte, 64),
	}
	h.register <- client

	go client.writePump()
	go client.readPump()

	welcome, _ := json.Marshal(OfferEvent{Type: "offer.connected"})
	select {
	case client.send <- welcome:
	default:
	}

	return client
}

// BroadcastToOrder sends a JSON message to all clients subscribed to a specific order.
func (h *Hub) BroadcastToOrder(orderID uuid.UUID, eventType string, offerData json.RawMessage) {
	msg := OfferEvent{Type: eventType, Offer: offerData}

	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("failed to marshal offer event", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients[orderID] {
		select {
		case client.send <- data:
		default:
			h.log.Warn("client send buffer full, disconnecting",
				"user_id", client.UserID.String(),
				"order_id", orderID.String(),
			)
			go func(c *Client) { h.unregister <- c }(client)
		}
	}
}
