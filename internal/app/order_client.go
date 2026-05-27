package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
	"github.com/companyofcreators/offer-service/pkg/header_auth"
)

// OrderClient communicates with the Order Service to validate customer ownership.
type OrderClient struct {
	baseURL    string
	httpClient *http.Client
	signer     *header_auth.HeaderSigner
	log        *slog.Logger
}

// NewOrderClient creates a new OrderClient.
func NewOrderClient(baseURL string, signer *header_auth.HeaderSigner, log *slog.Logger) *OrderClient {
	return &OrderClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		signer: signer,
		log:    log,
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

	// Sign the internal headers so the order-service can verify they came
	// from a trusted internal caller.
	c.signer.SignHeaders(req)

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

// AssignOrder updates the order status to "assigned" and sets the accepted offer ID.
// Retries up to 3 times with exponential backoff (1s, 2s, 4s) on failure.
func (c *OrderClient) AssignOrder(ctx context.Context, orderID, offerID, masterID uuid.UUID, finalPrice float64) error {
	url := fmt.Sprintf("%s/internal/orders/%s/assign", c.baseURL, orderID.String())

	var lastErr error
	const maxAttempts = 4 // 1 initial + 3 retries

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s
			select {
			case <-ctx.Done():
				return fmt.Errorf("assign order cancelled: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		body := map[string]interface{}{
			"offer_id":  offerID.String(),
			"final_price": finalPrice,
			"master_id": masterID.String(),
		}
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			lastErr = fmt.Errorf("failed to marshal assign body: %w", err)
			c.log.WarnContext(ctx, "assign order marshal failed, retrying",
				"attempt", attempt+1, "max_attempts", maxAttempts, "error", err)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
		if err != nil {
			lastErr = fmt.Errorf("failed to create assign request: %w", err)
			c.log.WarnContext(ctx, "assign order request creation failed, retrying",
				"attempt", attempt+1, "max_attempts", maxAttempts, "error", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to assign order: %w", err)
			c.log.WarnContext(ctx, "assign order HTTP call failed, retrying",
				"attempt", attempt+1, "max_attempts", maxAttempts, "error", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("order service returned status %d", resp.StatusCode)
			c.log.WarnContext(ctx, "assign order received non-OK status, retrying",
				"attempt", attempt+1, "max_attempts", maxAttempts, "status", resp.StatusCode)
			continue
		}

		resp.Body.Close()
		return nil
	}

	return fmt.Errorf("assign order failed after %d attempts: %w", maxAttempts, lastErr)
}
