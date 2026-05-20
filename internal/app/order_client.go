package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
)

// OrderClient communicates with the Order Service to validate customer ownership.
type OrderClient struct {
	baseURL    string
	httpClient *http.Client
	log        *slog.Logger
}

// NewOrderClient creates a new OrderClient.
func NewOrderClient(baseURL string, log *slog.Logger) *OrderClient {
	return &OrderClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		log: log,
	}
}

type orderOwnershipResponse struct {
	Order struct {
		CustomerID string `json:"customer_id"`
	} `json:"order"`
}

// ValidateOrderOwnership checks if the given user is the customer of the given order.
func (c *OrderClient) ValidateOrderOwnership(ctx context.Context, orderID, customerID uuid.UUID) error {
	url := fmt.Sprintf("%s/internal/orders/%s", c.baseURL, orderID.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-Id", customerID.String())
	req.Header.Set("X-User-Role", "customer")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.ErrorContext(ctx, "failed to call order service",
			"url", url,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to validate order ownership: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("order not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("order service returned status %d", resp.StatusCode)
	}

	var orderResp orderOwnershipResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return fmt.Errorf("failed to decode order service response: %w", err)
	}

	orderCustomerID, err := uuid.Parse(orderResp.Order.CustomerID)
	if err != nil {
		return fmt.Errorf("invalid customer_id in order service response: %w", err)
	}

	if orderCustomerID != customerID {
		return offerDomain.ErrUnauthorized
	}

	return nil
}
