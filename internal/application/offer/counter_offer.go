package offer

import (
	"context"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// CounterOfferUseCase handles the counter-proposal of a price by the customer.
type CounterOfferUseCase struct {
	service *offerDomain.Service
}

// NewCounterOfferUseCase creates a new CounterOfferUseCase.
func NewCounterOfferUseCase(service *offerDomain.Service) *CounterOfferUseCase {
	return &CounterOfferUseCase{service: service}
}

// CounterOfferInput represents the input for a counter-offer.
type CounterOfferInput struct {
	OfferID    uuid.UUID
	CustomerID uuid.UUID
	Price      float64
	Message    string
}

// Execute creates a counter-proposal negotiation event.
func (uc *CounterOfferUseCase) Execute(ctx context.Context, input CounterOfferInput) (*offerDomain.NegotiationEvent, error) {
	return uc.service.CounterOffer(ctx, input.OfferID, input.CustomerID, input.Price, input.Message)
}
