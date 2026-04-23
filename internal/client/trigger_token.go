package client

import (
	"net/http"
)

type TriggerTokenResponse struct {
	PublicAccessToken string `json:"publicAccessToken"`
	TriggerRunID      string `json:"triggerRunId"`
	ExpiresAt         string `json:"expiresAt"`
	StreamURL         string `json:"streamUrl"`
}

func (c *Client) GetTriggerToken(submissionID string) (*TriggerTokenResponse, error) {
	req, err := http.NewRequest("POST", c.BaseURL+"/v1/cli/submissions/"+submissionID+"/trigger-token", nil)
	if err != nil {
		return nil, err
	}
	var resp TriggerTokenResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
