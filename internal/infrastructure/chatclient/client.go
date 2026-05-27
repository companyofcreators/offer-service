package chatclient

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/google/uuid"
)

type ChatRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	MasterID   string `json:"master_id"`
}
type ChatResponse struct {
	ID string `json:"id"`
}
type Client struct {
	baseURL    string
	httpClient *http.Client
	hmacKey    []byte
}
func New(baseURL, hmacKey string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{Timeout: 5 * time.Second}, hmacKey: []byte(hmacKey)}
}
func (c *Client) CreateChat(ctx context.Context, orderID, customerID, masterID uuid.UUID) (string, error) {
	body, _ := json.Marshal(ChatRequest{OrderID: orderID.String(), CustomerID: customerID.String(), MasterID: masterID.String()})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/chats", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", customerID.String())
	req.Header.Set("X-User-Email", "system@diploma")
	req.Header.Set("X-User-Role", "user")
	c.signRequest(req)
	resp, err := c.httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode == 409 { return "", nil }
	if resp.StatusCode >= 300 { return "", fmt.Errorf("chat service %d", resp.StatusCode) }
	var chat ChatResponse
	json.NewDecoder(resp.Body).Decode(&chat)
	return chat.ID, nil
}
func (c *Client) signRequest(r *http.Request) {
	uid, email, role := r.Header.Get("X-User-Id"), r.Header.Get("X-User-Email"), r.Header.Get("X-User-Role")
	payload := strings.Join([]string{uid, email, role}, "|")
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(payload))
	r.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
