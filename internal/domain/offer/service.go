package offer

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// EventPublisher defines the contract for publishing domain events to Kafka.
type EventPublisher interface {
	PublishOfferCreated(ctx context.Context, offer *Offer, masterEmail string) error
	PublishOfferAccepted(ctx context.Context, offer *Offer, customerID uuid.UUID, customerEmail string) error
	PublishOfferRejected(ctx context.Context, offerID, orderID uuid.UUID) error
	PublishOfferWithdrawn(ctx context.Context, offer *Offer) error
	PublishOfferCountered(ctx context.Context, offerID, orderID, customerID uuid.UUID, proposedPrice float64) error
	PublishOfferUpdated(ctx context.Context, offer *Offer) error
}

// OrderClient defines the contract for communicating with the Order Service.
type OrderClient interface {
	// ValidateOrderOwnership checks if the given user is the customer of the given order.
	ValidateOrderOwnership(ctx context.Context, orderID, customerID uuid.UUID) error
	// AssignOrder updates the order status to "assigned" and sets the accepted offer.
	AssignOrder(ctx context.Context, orderID, offerID uuid.UUID) error
}

// Service implements the core business logic for offer negotiations.
type Service struct {
	offers      OfferRepository
	events      NegotiationEventRepository
	publisher   EventPublisher
	orderClient OrderClient
	logger      *slog.Logger
}

// NewService creates a new offer domain service.
func NewService(
	offers OfferRepository,
	events NegotiationEventRepository,
	publisher EventPublisher,
	orderClient OrderClient,
	logger *slog.Logger,
) *Service {
	return &Service{
		offers:      offers,
		events:      events,
		publisher:   publisher,
		orderClient: orderClient,
		logger:      logger,
	}
}

// SendOffer creates a new pending offer from a master for an order.
func (s *Service) SendOffer(ctx context.Context, orderID, masterID uuid.UUID, price float64, message string, masterEmail string) (*Offer, error) {
	if price <= 0 {
		return nil, ErrInvalidPrice
	}
	if message == "" {
		return nil, ErrEmptyMessage
	}
	if len(message) > 1000 {
		return nil, ErrMessageTooLong
	}

	existing, err := s.offers.FindPendingByMasterAndOrder(ctx, masterID, orderID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyPendingOffer
	}

	offer := NewOffer(orderID, masterID, price, message)

	if err := s.offers.Create(ctx, offer); err != nil {
		return nil, err
	}

	event := NewNegotiationEvent(
		offer.ID,
		offer.OrderID,
		masterID,
		EventOfferSent,
		"master",
		&price,
		message,
	)
	if err := s.events.Create(ctx, event); err != nil {
		return nil, err
	}

	if err := s.publisher.PublishOfferCreated(ctx, offer, masterEmail); err != nil {
		s.logger.Warn("failed to publish offer created event", "error", err, "offer_id", offer.ID)
	}

	return offer, nil
}

// WithdrawOffer allows a master to withdraw their own pending offer.
func (s *Service) WithdrawOffer(ctx context.Context, offerID, masterID uuid.UUID) (*Offer, error) {
	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	if !offer.CanBeModifiedBy(masterID) {
		return nil, ErrUnauthorized
	}
	if !offer.IsPending() {
		return nil, ErrOfferNotPending
	}

	if err := s.offers.UpdateStatus(ctx, offerID, OfferWithdrawn); err != nil {
		return nil, err
	}
	offer.Status = OfferWithdrawn

	event := NewNegotiationEvent(
		offerID,
		offer.OrderID,
		masterID,
		EventOfferWithdrawn,
		"master",
		nil,
		"",
	)
	if err := s.events.Create(ctx, event); err != nil {
		return nil, err
	}

	if err := s.publisher.PublishOfferWithdrawn(ctx, offer); err != nil {
		s.logger.Warn("failed to publish offer withdrawn event", "error", err, "offer_id", offer.ID)
	}

	return offer, nil
}

// AcceptOffer allows a customer to accept a pending offer.
// When an offer is accepted, all other pending offers for the same order are auto-rejected.
func (s *Service) AcceptOffer(ctx context.Context, offerID, customerID uuid.UUID, customerEmail string) (*Offer, error) {
	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	if err := s.orderClient.ValidateOrderOwnership(ctx, offer.OrderID, customerID); err != nil {
		return nil, err
	}

	if !offer.IsPending() {
		return nil, ErrOfferNotPending
	}

	if err := s.offers.UpdateStatus(ctx, offerID, OfferAccepted); err != nil {
		return nil, err
	}
	offer.Status = OfferAccepted

	if err := s.offers.RejectAllExcept(ctx, offer.OrderID, offerID); err != nil {
		return nil, err
	}

	// Update order status to assigned via direct API call.
	if err := s.orderClient.AssignOrder(ctx, offer.OrderID, offerID); err != nil {
		s.logger.Warn("failed to assign order via API", "error", err, "order_id", offer.OrderID, "offer_id", offerID)
	}

	event := NewNegotiationEvent(
		offerID,
		offer.OrderID,
		customerID,
		EventOfferAccepted,
		"customer",
		&offer.Price,
		"",
	)
	if err := s.events.Create(ctx, event); err != nil {
		return nil, err
	}

	if err := s.publisher.PublishOfferAccepted(ctx, offer, customerID, customerEmail); err != nil {
		s.logger.Warn("failed to publish offer accepted event", "error", err, "offer_id", offer.ID)
	}

	return offer, nil
}

// RejectOffer allows a customer to reject a pending offer.
func (s *Service) RejectOffer(ctx context.Context, offerID, customerID uuid.UUID) (*Offer, error) {
	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	if err := s.orderClient.ValidateOrderOwnership(ctx, offer.OrderID, customerID); err != nil {
		return nil, err
	}

	if !offer.IsPending() {
		return nil, ErrOfferNotPending
	}

	if err := s.offers.UpdateStatus(ctx, offerID, OfferRejected); err != nil {
		return nil, err
	}
	offer.Status = OfferRejected

	event := NewNegotiationEvent(
		offerID,
		offer.OrderID,
		customerID,
		EventOfferRejected,
		"customer",
		nil,
		"",
	)
	if err := s.events.Create(ctx, event); err != nil {
		return nil, err
	}

	if err := s.publisher.PublishOfferRejected(ctx, offerID, offer.OrderID); err != nil {
		s.logger.Warn("failed to publish offer rejected event", "error", err, "offer_id", offerID)
	}

	return offer, nil
}

// CounterOffer allows a customer to propose a different price in response to a pending offer.
// This does NOT create a new offer; it creates a negotiation event. The master must respond with a new offer.
func (s *Service) CounterOffer(ctx context.Context, offerID, customerID uuid.UUID, price float64, message string) (*NegotiationEvent, error) {
	if price <= 0 {
		return nil, ErrInvalidPrice
	}

	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	if err := s.orderClient.ValidateOrderOwnership(ctx, offer.OrderID, customerID); err != nil {
		return nil, err
	}

	if !offer.IsPending() {
		return nil, ErrOfferNotPending
	}

	event := NewNegotiationEvent(
		offerID,
		offer.OrderID,
		customerID,
		EventOfferCountered,
		"customer",
		&price,
		message,
	)
	if err := s.events.Create(ctx, event); err != nil {
		return nil, err
	}

	if err := s.publisher.PublishOfferCountered(ctx, offerID, offer.OrderID, customerID, price); err != nil {
		s.logger.Warn("failed to publish offer countered event", "error", err, "offer_id", offerID)
	}

	return event, nil
}

// GetOffer retrieves an offer by its ID.
func (s *Service) GetOffer(ctx context.Context, offerID uuid.UUID) (*Offer, error) {
	offer, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}
	return offer, nil
}

// ListOffersByOrder returns all offers for a given order.
func (s *Service) ListOffersByOrder(ctx context.Context, orderID uuid.UUID) ([]*Offer, error) {
	return s.offers.ListByOrder(ctx, orderID)
}

// ListOffersByMaster returns offers created by a master, with optional status filter and pagination.
func (s *Service) ListOffersByMaster(ctx context.Context, masterID uuid.UUID, status *OfferStatus, limit, offset int) ([]*Offer, int, error) {
	return s.offers.ListByMaster(ctx, masterID, status, limit, offset)
}

// GetOfferHistory returns the negotiation event history for a specific offer.
func (s *Service) GetOfferHistory(ctx context.Context, offerID uuid.UUID) ([]*NegotiationEvent, error) {
	// Verify the offer exists
	_, err := s.offers.FindByID(ctx, offerID)
	if err != nil {
		return nil, err
	}
	return s.events.ListByOffer(ctx, offerID)
}

// GetOrderHistory returns the full negotiation event history for an order.
func (s *Service) GetOrderHistory(ctx context.Context, orderID uuid.UUID) ([]*NegotiationEvent, error) {
	return s.events.ListByOrder(ctx, orderID)
}
