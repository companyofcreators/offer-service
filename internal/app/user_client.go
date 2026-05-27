package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type MasterProfile struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AvatarURL string    `json:"avatar_url"`
}

type MasterFullProfile struct {
	UserID          uuid.UUID `json:"user_id"`
	Rating          float64   `json:"rating"`
}

type FullUserProfileResponse struct {
	Profile       *MasterProfile     `json:"profile,omitempty"`
	MasterProfile *MasterFullProfile `json:"master_profile,omitempty"`
}

type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *UserClient) GetMasterProfile(ctx context.Context, userID uuid.UUID) (*MasterProfile, *MasterFullProfile, error) {
	url := fmt.Sprintf("%s/internal/users/%s", c.baseURL, userID.String())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("get profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil
	}
	var full FullUserProfileResponse
	json.NewDecoder(resp.Body).Decode(&full)
	return full.Profile, full.MasterProfile, nil
}
