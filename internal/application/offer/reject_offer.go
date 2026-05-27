package offer

import (
	"context"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// RejectOfferUseCase handles the rejection of an offer by the customer.
type RejectOfferUseCase struct {
	service *offerDomain.Service
}

// NewRejectOfferUseCase creates a new RejectOfferUseCase.
func NewRejectOfferUseCase(service *offerDomain.Service) *RejectOfferUseCase {
	return &RejectOfferUseCase{service: service}
}

// RejectOfferInput represents the input for rejecting an offer.
type RejectOfferInput struct {
	OfferID    uuid.UUID
	CustomerID uuid.UUID
}

// Execute rejects the offer.
func (uc *RejectOfferUseCase) Execute(ctx context.Context, input RejectOfferInput) (*offerDomain.Offer, error) {
	return uc.service.RejectOffer(ctx, input.OfferID, input.CustomerID)
}
