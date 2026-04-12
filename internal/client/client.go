package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootcraft-cn/cli/internal/version"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	Success bool            `json:"success"`
	Error   *apiError       `json:"error"`
	Data    json.RawMessage `json:"data"`
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Code)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "bootcraft-cli/"+version.Version)
	return c.HTTPClient.Do(req)
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		if resp.StatusCode >= 400 {
			return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
		}
		return fmt.Errorf("parsing response: %w", err)
	}

	if apiResp.Error != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       apiResp.Error.Code,
			Message:    apiResp.Error.Message,
		}
	}

	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	if out != nil && apiResp.Data != nil {
		return json.Unmarshal(apiResp.Data, out)
	}
	// If no envelope (some endpoints return data directly), parse the full body
	if out != nil && apiResp.Data == nil && !apiResp.Success {
		return json.Unmarshal(body, out)
	}
	return nil
}
