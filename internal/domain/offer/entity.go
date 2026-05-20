package offer

import (
	"time"

	"github.com/google/uuid"
)

// OfferStatus represents the current state of an offer.
type OfferStatus string

const (
	OfferPending   OfferStatus = "pending"
	OfferAccepted  OfferStatus = "accepted"
	OfferRejected  OfferStatus = "rejected"
	OfferWithdrawn OfferStatus = "withdrawn"
)

func (s OfferStatus) IsValid() bool {
	switch s {
	case OfferPending, OfferAccepted, OfferRejected, OfferWithdrawn:
		return true
	default:
		return false
	}
}

func (s OfferStatus) IsTerminal() bool {
	return s == OfferAccepted || s == OfferRejected || s == OfferWithdrawn
}

func (s OfferStatus) String() string {
	return string(s)
}

// Offer represents a master's price proposal for an order.
// Offers are immutable once created; only the status changes.
type Offer struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	MasterID  uuid.UUID
	Price     float64
	Message   string
	Status    OfferStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewOffer creates a new pending offer.
func NewOffer(orderID, masterID uuid.UUID, price float64, message string) *Offer {
	now := time.Now().UTC()
	return &Offer{
		ID:        uuid.New(),
		OrderID:   orderID,
		MasterID:  masterID,
		Price:     price,
		Message:   message,
		Status:    OfferPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsPending returns true if the offer is still pending.
func (o *Offer) IsPending() bool {
	return o.Status == OfferPending
}

// CanBeModifiedBy returns true if the given user can modify this offer.
func (o *Offer) CanBeModifiedBy(userID uuid.UUID) bool {
	return o.MasterID == userID
}

// NegotiationEventType represents the type of a negotiation event.
type NegotiationEventType string

const (
	EventOfferSent      NegotiationEventType = "offer.sent"
	EventOfferCountered NegotiationEventType = "offer.countered"
	EventOfferAccepted  NegotiationEventType = "offer.accepted"
	EventOfferRejected  NegotiationEventType = "offer.rejected"
	EventOfferWithdrawn NegotiationEventType = "offer.withdrawn"
)

func (t NegotiationEventType) String() string {
	return string(t)
}

// NegotiationEvent represents an immutable event in the negotiation timeline.
// Every action (send, accept, reject, withdraw, counter) creates a new event.
type NegotiationEvent struct {
	ID        uuid.UUID
	OfferID   uuid.UUID
	OrderID   uuid.UUID
	Type      NegotiationEventType
	ActorID   uuid.UUID
	ActorRole string
	Price     *float64
	Message   string
	CreatedAt time.Time
}

// NewNegotiationEvent creates a new negotiation event.
func NewNegotiationEvent(
	offerID, orderID, actorID uuid.UUID,
	eventType NegotiationEventType,
	actorRole string,
	price *float64,
	message string,
) *NegotiationEvent {
	return &NegotiationEvent{
		ID:        uuid.New(),
		OfferID:   offerID,
		OrderID:   orderID,
		Type:      eventType,
		ActorID:   actorID,
		ActorRole: actorRole,
		Price:     price,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
}
