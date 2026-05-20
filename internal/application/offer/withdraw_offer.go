package offer

import (
	"context"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// WithdrawOfferUseCase handles the withdrawal of a pending offer by its master.
type WithdrawOfferUseCase struct {
	service *offerDomain.Service
}

// NewWithdrawOfferUseCase creates a new WithdrawOfferUseCase.
func NewWithdrawOfferUseCase(service *offerDomain.Service) *WithdrawOfferUseCase {
	return &WithdrawOfferUseCase{service: service}
}

// WithdrawOfferInput represents the input for withdrawing an offer.
type WithdrawOfferInput struct {
	OfferID  uuid.UUID
	MasterID uuid.UUID
}

// Execute withdraws the offer.
func (uc *WithdrawOfferUseCase) Execute(ctx context.Context, input WithdrawOfferInput) (*offerDomain.Offer, error) {
	return uc.service.WithdrawOffer(ctx, input.OfferID, input.MasterID)
}
