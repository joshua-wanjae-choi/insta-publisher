package igclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://graph.facebook.com"

type Client struct {
	httpClient *http.Client
	baseURL    string
	version    string
	accountID  string
	token      string
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
	query.Set("access_token", c.token)
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
	formData.Set("access_token", c.token)

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
