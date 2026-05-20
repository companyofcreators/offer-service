package offer

import (
	"context"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// SendOfferUseCase handles the creation of a new offer by a master.
type SendOfferUseCase struct {
	service *offerDomain.Service
}

// NewSendOfferUseCase creates a new SendOfferUseCase.
func NewSendOfferUseCase(service *offerDomain.Service) *SendOfferUseCase {
	return &SendOfferUseCase{service: service}
}

// SendOfferInput represents the input for sending an offer.
type SendOfferInput struct {
	OrderID     uuid.UUID
	MasterID    uuid.UUID
	Price       float64
	Message     string
	MasterEmail string
}

// Execute creates a new offer.
func (uc *SendOfferUseCase) Execute(ctx context.Context, input SendOfferInput) (*offerDomain.Offer, error) {
	return uc.service.SendOffer(ctx, input.OrderID, input.MasterID, input.Price, input.Message, input.MasterEmail)
}
