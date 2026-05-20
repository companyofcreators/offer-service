package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/companyofcreators/offer-service/internal/domain/offer"
)

// SendOfferRequest represents the request body for creating a new offer.
type SendOfferRequest struct {
	OrderID uuid.UUID `json:"order_id" validate:"required,uuid"`
	Price   float64   `json:"price" validate:"required,gt=0"`
	Message string    `json:"message" validate:"required,min=1,max=1000"`
}

// CounterOfferRequest represents the request body for a counter-proposal.
type CounterOfferRequest struct {
	Price   float64 `json:"price" validate:"required,gt=0"`
	Message string  `json:"message" validate:"max=1000"`
}

// OfferResponse represents the API response for an offer.
type OfferResponse struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	MasterID  uuid.UUID `json:"master_id"`
	Price     float64   `json:"price"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// OfferListResponse represents a paginated list of offers.
type OfferListResponse struct {
	Offers []OfferResponse `json:"offers"`
	Total  int             `json:"total"`
}

// NegotiationEventResponse represents the API response for a negotiation event.
type NegotiationEventResponse struct {
	ID        uuid.UUID `json:"id"`
	OfferID   uuid.UUID `json:"offer_id"`
	Type      string    `json:"type"`
	ActorID   uuid.UUID `json:"actor_id"`
	ActorRole string    `json:"actor_role"`
	Price     *float64  `json:"price,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// NegotiationHistoryResponse represents the full history of negotiation events.
type NegotiationHistoryResponse struct {
	Events []NegotiationEventResponse `json:"events"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// toOfferResponse converts a domain offer to an API response.
func toOfferResponse(o *offer.Offer) OfferResponse {
	return OfferResponse{
		ID:        o.ID,
		OrderID:   o.OrderID,
		MasterID:  o.MasterID,
		Price:     o.Price,
		Message:   o.Message,
		Status:    string(o.Status),
		CreatedAt: o.CreatedAt,
	}
}

// toOfferListResponse converts a slice of domain offers to an API response.
func toOfferListResponse(offers []*offer.Offer, total int) OfferListResponse {
	items := make([]OfferResponse, 0, len(offers))
	for _, o := range offers {
		items = append(items, toOfferResponse(o))
	}
	return OfferListResponse{
		Offers: items,
		Total:  total,
	}
}

// toNegotiationEventResponse converts a domain negotiation event to an API response.
func toNegotiationEventResponse(e *offer.NegotiationEvent) NegotiationEventResponse {
	resp := NegotiationEventResponse{
		ID:        e.ID,
		OfferID:   e.OfferID,
		Type:      string(e.Type),
		ActorID:   e.ActorID,
		ActorRole: e.ActorRole,
		Price:     e.Price,
		Message:   e.Message,
		CreatedAt: e.CreatedAt,
	}
	return resp
}

// toNegotiationHistoryResponse converts a slice of domain events to an API response.
func toNegotiationHistoryResponse(events []*offer.NegotiationEvent) NegotiationHistoryResponse {
	items := make([]NegotiationEventResponse, 0, len(events))
	for _, e := range events {
		items = append(items, toNegotiationEventResponse(e))
	}
	return NegotiationHistoryResponse{
		Events: items,
	}
}
