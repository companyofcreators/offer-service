package offer

import (
	"context"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// AcceptOfferUseCase handles the acceptance of an offer by the customer.
type AcceptOfferUseCase struct {
	service *offerDomain.Service
}

// NewAcceptOfferUseCase creates a new AcceptOfferUseCase.
func NewAcceptOfferUseCase(service *offerDomain.Service) *AcceptOfferUseCase {
	return &AcceptOfferUseCase{service: service}
}

// AcceptOfferInput represents the input for accepting an offer.
type AcceptOfferInput struct {
	OfferID    uuid.UUID
	CustomerID uuid.UUID
}

// Execute accepts the offer.
func (uc *AcceptOfferUseCase) Execute(ctx context.Context, input AcceptOfferInput) (*offerDomain.Offer, error) {
	return uc.service.AcceptOffer(ctx, input.OfferID, input.CustomerID)
}
