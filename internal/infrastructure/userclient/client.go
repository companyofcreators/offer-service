package userclient

import (
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

type MasterProfile struct {
	ID uuid.UUID `json:"id"`
	FirstName string `json:"first_name"`
	LastName string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}
type MasterFullProfile struct {
	UserID uuid.UUID `json:"user_id"`
	Rating float64 `json:"rating"`
}
type FullUserProfileResponse struct {
	Profile *MasterProfile `json:"profile,omitempty"`
	MasterProfile *MasterFullProfile `json:"master_profile,omitempty"`
}
type Client struct {
	baseURL string
	httpClient *http.Client
	hmacKey []byte
}
func New(baseURL, hmacKey string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		hmacKey: []byte(hmacKey),
	}
}
func (c *Client) signRequest(r *http.Request) {
	if len(c.hmacKey) == 0 { return }
	uid := r.Header.Get("X-User-Id")
	email := r.Header.Get("X-User-Email")
	role := r.Header.Get("X-User-Role")
	payload := strings.Join([]string{uid, email, role}, "|")
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(payload))
	r.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
func (c *Client) GetMasterProfile(ctx context.Context, userID uuid.UUID) (*MasterProfile, *MasterFullProfile, error) {
	url := fmt.Sprintf("%s/internal/users/%s", c.baseURL, userID.String())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("X-User-Id", userID.String())
	req.Header.Set("X-User-Email", "system@diploma")
	req.Header.Set("X-User-Role", "admin")
	c.signRequest(req)
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, nil, fmt.Errorf("get profile: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, nil, nil }
	var full FullUserProfileResponse
	json.NewDecoder(resp.Body).Decode(&full)
	return full.Profile, full.MasterProfile, nil
}
