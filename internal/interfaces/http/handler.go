package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	offerApp "github.com/companyofcreators/offer-service/internal/application/offer"
	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
	"github.com/companyofcreators/offer-service/pkg"
)

// RoleChecker checks if a user has a specific role by querying the user-service database.
type RoleChecker interface {
	HasRole(ctx context.Context, userID, role string) (bool, error)
}

// Handler handles HTTP requests for the offer service.
type Handler struct {
	sendOffer     *offerApp.SendOfferUseCase
	withdrawOffer *offerApp.WithdrawOfferUseCase
	acceptOffer   *offerApp.AcceptOfferUseCase
	rejectOffer   *offerApp.RejectOfferUseCase
	counterOffer  *offerApp.CounterOfferUseCase
	service       *offerDomain.Service
	roleChecker   RoleChecker
	validate      *validator.Validate
	log           *slog.Logger
}

// NewHandler creates a new HTTP handler.
func NewHandler(
	sendOffer *offerApp.SendOfferUseCase,
	withdrawOffer *offerApp.WithdrawOfferUseCase,
	acceptOffer *offerApp.AcceptOfferUseCase,
	rejectOffer *offerApp.RejectOfferUseCase,
	counterOffer *offerApp.CounterOfferUseCase,
	service *offerDomain.Service,
	roleChecker RoleChecker,
	log *slog.Logger,
) *Handler {
	return &Handler{
		sendOffer:     sendOffer,
		withdrawOffer: withdrawOffer,
		acceptOffer:   acceptOffer,
		rejectOffer:   rejectOffer,
		counterOffer:  counterOffer,
		service:       service,
		roleChecker:   roleChecker,
		validate:      validator.New(),
		log:           log,
	}
}

// SendOffer handles POST /internal/offers
func (h *Handler) SendOffer(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "отсутствует или недействителен заголовок user id")
		return
	}

	isMaster, err := h.roleChecker.HasRole(r.Context(), userID.String(), "master")
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to check master role", "error", err.Error())
		h.writeError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя")
		return
	}
	if !isMaster {
		h.writeError(w, http.StatusForbidden, "только мастера могут отправлять предложения")
		return
	}

	var req SendOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if verrs := pkg.ValidateStruct(req); verrs != nil {
		pkg.WriteValidationErrors(w, verrs)
		return
	}

	masterEmail := r.Header.Get("X-User-Email")

	offer, err := h.sendOffer.Execute(r.Context(), offerApp.SendOfferInput{
		OrderID:     req.OrderID,
		MasterID:    userID,
		Price:       req.Price,
		Message:     req.Message,
		MasterEmail: masterEmail,
	})
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusCreated, toOfferResponse(offer))
}

// WithdrawOffer handles POST /internal/offers/{id}/withdraw
func (h *Handler) WithdrawOffer(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "отсутствует или недействителен заголовок user id")
		return
	}

	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	offer, err := h.withdrawOffer.Execute(r.Context(), offerApp.WithdrawOfferInput{
		OfferID:  offerID,
		MasterID: userID,
	})
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toOfferResponse(offer))
}

// AcceptOffer handles POST /internal/offers/{id}/accept
func (h *Handler) AcceptOffer(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "отсутствует или недействителен заголовок user id")
		return
	}

	isCustomer, err := h.roleChecker.HasRole(r.Context(), userID.String(), "user")
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to check user role for accept", "error", err.Error())
		h.writeError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя")
		return
	}
	if !isCustomer {
		h.writeError(w, http.StatusForbidden, "только заказчики могут принимать предложения")
		return
	}

	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	customerEmail := r.Header.Get("X-User-Email")

	offer, err := h.acceptOffer.Execute(r.Context(), offerApp.AcceptOfferInput{
		OfferID:       offerID,
		CustomerID:    userID,
		CustomerEmail: customerEmail,
	})
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toOfferResponse(offer))
}

// RejectOffer handles POST /internal/offers/{id}/reject
func (h *Handler) RejectOffer(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "отсутствует или недействителен заголовок user id")
		return
	}

	isCustomer, err := h.roleChecker.HasRole(r.Context(), userID.String(), "user")
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to check user role for reject", "error", err.Error())
		h.writeError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя")
		return
	}
	if !isCustomer {
		h.writeError(w, http.StatusForbidden, "только заказчики могут отклонять предложения")
		return
	}

	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	offer, err := h.rejectOffer.Execute(r.Context(), offerApp.RejectOfferInput{
		OfferID:    offerID,
		CustomerID: userID,
	})
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toOfferResponse(offer))
}

// CounterOffer handles POST /internal/offers/{id}/counter
func (h *Handler) CounterOffer(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "отсутствует или недействителен заголовок user id")
		return
	}

	isCustomer, err := h.roleChecker.HasRole(r.Context(), userID.String(), "user")
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to check user role for counter", "error", err.Error())
		h.writeError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя")
		return
	}
	if !isCustomer {
		isMaster, err := h.roleChecker.HasRole(r.Context(), userID.String(), "master")
		if err != nil {
			h.log.ErrorContext(r.Context(), "failed to check master role for counter", "error", err.Error())
			h.writeError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя")
			return
		}
		if !isMaster {
			h.writeError(w, http.StatusForbidden, "только заказчики и мастера могут делать контр-предложения")
			return
		}
	}

	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	var req CounterOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if verrs := pkg.ValidateStruct(req); verrs != nil {
		pkg.WriteValidationErrors(w, verrs)
		return
	}

	event, err := h.counterOffer.Execute(r.Context(), offerApp.CounterOfferInput{
		OfferID:    offerID,
		CustomerID: userID,
		Price:      req.Price,
		Message:    req.Message,
	})
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toNegotiationEventResponse(event))
}

// GetOffer handles GET /internal/offers/{id}
func (h *Handler) GetOffer(w http.ResponseWriter, r *http.Request) {
	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	offer, err := h.service.GetOffer(r.Context(), offerID)
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toOfferResponse(offer))
}

// ListOffers handles GET /internal/offers
func (h *Handler) ListOffers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	orderIDStr := query.Get("order_id")
	masterIDStr := query.Get("master_id")
	statusStr := query.Get("status")

	if orderIDStr != "" {
		orderID, err := uuid.Parse(orderIDStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "недействительный order_id")
			return
		}

		offers, err := h.service.ListOffersByOrder(r.Context(), orderID)
		if err != nil {
			h.handleDomainError(w, r.Context(), err)
			return
		}

		h.writeJSON(w, http.StatusOK, toOfferListResponse(offers, len(offers)))
		return
	}

	if masterIDStr != "" {
		masterID, err := uuid.Parse(masterIDStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "недействительный master_id")
			return
		}

		var statusFilter *offerDomain.OfferStatus
		if statusStr != "" {
			s := offerDomain.OfferStatus(statusStr)
			if !s.IsValid() {
				h.writeError(w, http.StatusBadRequest, "недопустимый фильтр статуса")
				return
			}
			statusFilter = &s
		}

		limit := 20
		offset := 0
		if l := query.Get("limit"); l != "" {
			var err error
			limit, err = strconv.Atoi(l)
			if err != nil {
				slog.Warn("invalid limit query param, using default", "value", l, "error", err)
				limit = 0
			}
		}
		if o := query.Get("offset"); o != "" {
			var err error
			offset, err = strconv.Atoi(o)
			if err != nil {
				slog.Warn("invalid offset query param, using default", "value", o, "error", err)
				offset = 0
			}
		}

		offers, total, err := h.service.ListOffersByMaster(r.Context(), masterID, statusFilter, limit, offset)
		if err != nil {
			h.handleDomainError(w, r.Context(), err)
			return
		}

		h.writeJSON(w, http.StatusOK, toOfferListResponse(offers, total))
		return
	}

	h.writeError(w, http.StatusBadRequest, "требуется параметр order_id или master_id")
}

// GetOfferHistory handles GET /internal/offers/{id}/history
func (h *Handler) GetOfferHistory(w http.ResponseWriter, r *http.Request) {
	offerID, err := extractOfferID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID предложения")
		return
	}

	events, err := h.service.GetOfferHistory(r.Context(), offerID)
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toNegotiationHistoryResponse(events))
}

// GetOrderHistory handles GET /internal/orders/{id}/history
func (h *Handler) GetOrderHistory(w http.ResponseWriter, r *http.Request) {
	orderID, err := extractOrderID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный ID заказа")
		return
	}

	events, err := h.service.GetOrderHistory(r.Context(), orderID)
	if err != nil {
		h.handleDomainError(w, r.Context(), err)
		return
	}

	h.writeJSON(w, http.StatusOK, toNegotiationHistoryResponse(events))
}

// Health handles GET /internal/health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "offer-service"})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.ErrorContext(context.Background(), "failed to encode response", "error", err.Error())
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, ErrorResponse{
		Error:   statusText(status),
		Message: message,
	})
}

func (h *Handler) handleDomainError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, offerDomain.ErrOfferNotFound):
		h.writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, offerDomain.ErrOfferNotPending):
		h.writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, offerDomain.ErrUnauthorized):
		h.writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, offerDomain.ErrAlreadyPendingOffer):
		h.writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, offerDomain.ErrInvalidPrice):
		h.writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, offerDomain.ErrEmptyMessage):
		h.writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, offerDomain.ErrMessageTooLong):
		h.writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.log.ErrorContext(ctx, "unexpected error", "error", err.Error())
		h.writeError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
	}
}

func statusText(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "некорректный запрос"
	case http.StatusUnauthorized:
		return "не авторизован"
	case http.StatusForbidden:
		return "доступ запрещён"
	case http.StatusNotFound:
		return "не найдено"
	case http.StatusConflict:
		return "конфликт"
	case http.StatusUnprocessableEntity:
		return "ошибка валидации"
	case http.StatusTooManyRequests:
		return "слишком много запросов"
	case http.StatusInternalServerError:
		return "внутренняя ошибка сервера"
	default:
		return "ошибка"
	}
}

func extractUserID(r *http.Request) (uuid.UUID, error) {
	id := r.Header.Get("X-User-Id")
	if id == "" {
		return uuid.Nil, errors.New("missing X-User-Id header")
	}
	return uuid.Parse(id)
}

func extractOfferID(r *http.Request) (uuid.UUID, error) {
	id := r.PathValue("id")
	if id == "" {
		return uuid.Nil, errors.New("missing offer id")
	}
	return uuid.Parse(id)
}

func extractOrderID(r *http.Request) (uuid.UUID, error) {
	id := r.PathValue("id")
	if id == "" {
		return uuid.Nil, errors.New("missing order id")
	}
	return uuid.Parse(id)
}
