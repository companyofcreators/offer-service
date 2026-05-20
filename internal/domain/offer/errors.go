package offer

import "errors"

var (
	ErrOfferNotFound       = errors.New("offer not found")
	ErrOfferNotPending     = errors.New("offer is not in pending status")
	ErrUnauthorized        = errors.New("user is not authorized to perform this action")
	ErrAlreadyPendingOffer = errors.New("master already has a pending offer for this order")
	ErrInvalidPrice        = errors.New("price must be greater than zero")
	ErrEmptyMessage        = errors.New("message must not be empty")
	ErrMessageTooLong      = errors.New("message exceeds maximum length of 1000 characters")
	ErrInvalidOfferID      = errors.New("invalid offer ID")
	ErrInvalidOrderID      = errors.New("invalid order ID")
)
