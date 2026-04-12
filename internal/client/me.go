package client

import (
	"net/http"
)

type MeResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	GithubUsername string `json:"githubUsername"`
	Name           string `json:"name"`
}

func (c *Client) GetMe() (*MeResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/cli/me", nil)
	if err != nil {
		return nil, err
	}
	var resp MeResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
