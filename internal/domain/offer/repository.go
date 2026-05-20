package offer

import (
	"context"

	"github.com/google/uuid"
)

// OfferRepository defines the persistence contract for offers.
type OfferRepository interface {
	// Create inserts a new offer.
	Create(ctx context.Context, o *Offer) error

	// FindByID retrieves a single offer by its ID.
	FindByID(ctx context.Context, id uuid.UUID) (*Offer, error)

	// ListByOrder returns all offers for a given order.
	ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*Offer, error)

	// ListByMaster returns offers created by a master, with optional status filter and pagination.
	ListByMaster(ctx context.Context, masterID uuid.UUID, status *OfferStatus, limit, offset int) ([]*Offer, int, error)

	// UpdateStatus changes the status of an offer.
	UpdateStatus(ctx context.Context, id uuid.UUID, status OfferStatus) error

	// RejectAllExcept rejects all pending offers for an order except the specified one.
	// Must be executed atomically (in a transaction).
	RejectAllExcept(ctx context.Context, orderID uuid.UUID, exceptOfferID uuid.UUID) error

	// CountPending returns the number of pending offers for an order.
	CountPending(ctx context.Context, orderID uuid.UUID) (int, error)

	// FindPendingByMasterAndOrder finds a pending offer by a specific master for a specific order.
	FindPendingByMasterAndOrder(ctx context.Context, masterID, orderID uuid.UUID) (*Offer, error)
}

// NegotiationEventRepository defines the persistence contract for negotiation events.
type NegotiationEventRepository interface {
	// Create inserts a new negotiation event.
	Create(ctx context.Context, e *NegotiationEvent) error

	// ListByOrder returns all negotiation events for a given order, ordered by creation time.
	ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*NegotiationEvent, error)

	// ListByOffer returns all negotiation events for a given offer, ordered by creation time.
	ListByOffer(ctx context.Context, offerID uuid.UUID) ([]*NegotiationEvent, error)
}
