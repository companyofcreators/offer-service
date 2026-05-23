package offer

import "errors"

var (
	ErrOfferNotFound       = errors.New("предложение не найдено")
	ErrOfferNotPending     = errors.New("предложение не в статусе ожидания")
	ErrUnauthorized        = errors.New("пользователь не авторизован для этого действия")
	ErrAlreadyPendingOffer = errors.New("у мастера уже есть ожидающее предложение по этому заказу")
	ErrInvalidPrice        = errors.New("цена должна быть больше нуля")
	ErrEmptyMessage        = errors.New("сообщение не может быть пустым")
	ErrMessageTooLong      = errors.New("сообщение превышает максимальную длину в 1000 символов")
	ErrInvalidOfferID      = errors.New("недействительный ID предложения")
	ErrInvalidOrderID      = errors.New("недействительный ID заказа")
	ErrAssignOrderFailed   = errors.New("не удалось назначить заказ")
)
