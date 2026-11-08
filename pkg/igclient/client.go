package igclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const defaultBaseURL = "https://graph.facebook.com"

type Client struct {
	httpClient *http.Client
	baseURL    string
	version    string
	accountID  string
	token      string
	tokenMu    sync.RWMutex
}

type TokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type APIErrorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FBTraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

func New(accountID, token, version string) *Client {
	if version == "" {
		version = "v21.0"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    defaultBaseURL,
		version:    version,
		accountID:  accountID,
		token:      token,
	}
}

func (c *Client) buildURL(endpoint string, query url.Values) string {
	if query == nil {
		query = url.Values{}
	}
	c.tokenMu.RLock()
	curToken := c.token
	c.tokenMu.RUnlock()
	query.Set("access_token", curToken)
	return fmt.Sprintf("%s/%s/%s?%s", c.baseURL, c.version, endpoint, query.Encode())
}

func (c *Client) get(ctx context.Context, endpoint string, query url.Values, target any) error {
	fullURL := c.buildURL(endpoint, query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("create GET request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute GET request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("graph api error [%d / %d]: %s", apiErr.Error.Code, apiErr.Error.ErrorSubcode, apiErr.Error.Message)
		}
		return fmt.Errorf("graph api HTTP error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("parse json response: %w", err)
		}
	}
	return nil
}

func (c *Client) post(ctx context.Context, endpoint string, formData url.Values, target any) error {
	if formData == nil {
		formData = url.Values{}
	}
	c.tokenMu.RLock()
	curToken := c.token
	c.tokenMu.RUnlock()
	formData.Set("access_token", curToken)

	fullURL := fmt.Sprintf("%s/%s/%s", c.baseURL, c.version, endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("graph api error [%d / %d]: %s", apiErr.Error.Code, apiErr.Error.ErrorSubcode, apiErr.Error.Message)
		}
		return fmt.Errorf("graph api HTTP error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("parse json response: %w", err)
		}
	}
	return nil
}

// GetAccountInfo checks if token and account ID are valid
func (c *Client) GetAccountInfo(ctx context.Context) (map[string]any, error) {
	endpoint := c.accountID
	query := url.Values{}
	query.Set("fields", "id,username,name,biography,profile_picture_url")

	var res map[string]any
	if err := c.get(ctx, endpoint, query, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetToken returns the current active token
func (c *Client) GetToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.token
}

// UpdateToken updates the active token in memory
func (c *Client) UpdateToken(newToken string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = newToken
}

// RefreshToken exchanges an existing long-lived token for an extended one (adds another 60 days)
func (c *Client) RefreshToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	currentToken := c.token
	c.tokenMu.RUnlock()

	if currentToken == "" {
		return "", fmt.Errorf("no access token available to refresh")
	}

	refreshURL := fmt.Sprintf("%s/%s/oauth/access_token", c.baseURL, c.version)
	query := url.Values{}
	query.Set("grant_type", "ig_refresh_token")
	query.Set("access_token", currentToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?%s", refreshURL, query.Encode()), nil)
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read refresh response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("refresh token error [%d]: %s", apiErr.Error.Code, apiErr.Error.Message)
		}
		return "", fmt.Errorf("refresh token HTTP error [%d]: %s", resp.StatusCode, string(body))
	}

	var res TokenRefreshResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("unmarshal refresh response: %w", err)
	}

	if res.AccessToken == "" {
		return "", fmt.Errorf("no access_token in refresh response")
	}

	c.UpdateToken(res.AccessToken)
	return res.AccessToken, nil
}
