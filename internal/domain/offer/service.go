package offer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// Broadcaster defines the contract for broadcasting offer events via WebSocket.
type Broadcaster interface {
	BroadcastToOrder(orderID uuid.UUID, eventType string, data json.RawMessage)
}

// ChatCreator creates a chat between customer and master when an offer is accepted.
type ChatCreator interface {
	CreateChat(ctx context.Context, orderID, customerID, masterID uuid.UUID) (string, error)
}

// EventPublisher defines the contract for publishing domain events to Kafka.
type EventPublisher interface {
	PublishOfferCreated(ctx context.Context, offer *Offer, masterEmail string) error
	PublishOfferAccepted(ctx context.Context, offer *Offer, customerID uuid.UUID, customerEmail string, masterEmail string) error
	PublishOfferRejected(ctx context.Context, offer *Offer) error
	PublishOfferWithdrawn(ctx context.Context, offer *Offer) error
	PublishOfferCountered(ctx context.Context, offer *Offer, customerID uuid.UUID, proposedPrice float64, message string) error
	PublishOfferUpdated(ctx context.Context, offer *Offer) error
}

// OrderClient defines the contract for communicating with the Order Service.
type OrderClient interface {
	// ValidateOrderOwnership checks if the given user is the customer of the given order.
	ValidateOrderOwnership(ctx context.Context, orderID, customerID uuid.UUID) error
	// AssignOrder updates the order status to "assigned" and sets the accepted offer.
	AssignOrder(ctx context.Context, orderID, offerID, masterID uuid.UUID, finalPrice float64) error
}

// Service implements the core business logic for offer negotiations.
type Service struct {
	offers      OfferRepository
	events      NegotiationEventRepository
	publisher   EventPublisher
	orderClient OrderClient
	broadcaster Broadcaster
	chatCreator ChatCreator
	logger      *slog.Logger
}

// NewService creates a new offer domain service.
func NewService(
	offers OfferRepository,
	events NegotiationEventRepository,
	publisher EventPublisher,
	orderClient OrderClient,
	broadcaster Broadcaster,
	chatCreator ChatCreator,
	logger *slog.Logger,
) *Service {
	return &Service{
		offers:      offers,
		events:      events,
		publisher:   publisher,
		orderClient: orderClient,
		broadcaster: broadcaster,
		chatCreator: chatCreator,
		logger:      logger,
	}
}

// BroadcastOffer re-broadcasts an offer with enriched data (master_name etc) over WebSocket.
func (s *Service) BroadcastOffer(ctx context.Context, orderID uuid.UUID, offer *Offer, enriched interface{}) {
	s.broadcastOffer(orderID, "offer.created", enriched)
}

func (s *Service) broadcastOffer(orderID uuid.UUID, eventType string, payload interface{}) {
	if s.broadcaster == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("failed to marshal ws broadcast", "error", err)
		return
	}
	s.broadcaster.BroadcastToOrder(orderID, eventType, data)
}

// SendOffer creates a new pending offer from a master for an order.
func (s *Service) SendOffer(ctx context.Context, orderID, masterID uuid.UUID, price float64, message string, masterEmail string) (*Offer, error) {
	if price <= 0 {
		return nil, ErrInvalidPrice
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
	offer.MasterEmail = masterEmail

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

	s.broadcastOffer(orderID, "offer.created", offer)
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

	s.broadcastOffer(offer.OrderID, "offer.updated", offer)
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
	if err := s.orderClient.AssignOrder(ctx, offer.OrderID, offerID, offer.MasterID, offer.Price); err != nil {
		s.logger.Error("failed to assign order, rolling back offer status",
			"error", err, "offer_id", offerID, "order_id", offer.OrderID)
		if rollbackErr := s.offers.UpdateStatus(ctx, offerID, OfferPending); rollbackErr != nil {
			s.logger.Error("CRITICAL: failed to rollback after AssignOrder failure",
				"error", rollbackErr, "offer_id", offerID)
		}
		return nil, fmt.Errorf("не удалось назначить заказ: %w", err)
	}

	// Auto-create chat between customer and master
	if s.chatCreator != nil {
		chatID, chatErr := s.chatCreator.CreateChat(ctx, offer.OrderID, customerID, offer.MasterID)
		if chatErr != nil {
			s.logger.Warn("failed to auto-create chat after accept", "error", chatErr, "order_id", offer.OrderID)
		} else if chatID != "" {
			s.logger.Info("chat auto-created after offer acceptance", "chat_id", chatID, "order_id", offer.OrderID)
		}
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

	if err := s.publisher.PublishOfferAccepted(ctx, offer, customerID, customerEmail, offer.MasterEmail); err != nil {
		s.logger.Warn("failed to publish offer accepted event", "error", err, "offer_id", offer.ID)
	}

	s.broadcastOffer(offer.OrderID, "offer.updated", offer)
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

	if err := s.publisher.PublishOfferRejected(ctx, offer); err != nil {
		s.logger.Warn("failed to publish offer rejected event", "error", err, "offer_id", offerID)
	}

	s.broadcastOffer(offer.OrderID, "offer.updated", offer)
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

	if err := s.publisher.PublishOfferCountered(ctx, offer, customerID, price, message); err != nil {
		s.logger.Warn("failed to publish offer countered event", "error", err, "offer_id", offerID)
	}

	s.broadcastOffer(offer.OrderID, "offer.countered", event)
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
