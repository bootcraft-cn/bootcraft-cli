package client

import (
	"net/http"
)

type InitAuthResponse struct {
	Code      string `json:"code"`
	AuthURL   string `json:"authUrl"`
	ExpiresIn int    `json:"expiresIn"`
}

type PollAuthResponse struct {
	Status   string `json:"status"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
}

func (c *Client) InitCLIAuth() (*InitAuthResponse, error) {
	req, err := http.NewRequest("POST", c.BaseURL+"/v1/cli-auth/init", nil)
	if err != nil {
		return nil, err
	}
	var resp InitAuthResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCLIAuthToken(code string) (*PollAuthResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/cli-auth/token?code="+code, nil)
	if err != nil {
		return nil, err
	}
	var resp PollAuthResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
