package client

import (
	"net/http"
)

type SubmissionStatusResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	StageSlug     string `json:"stageSlug"`
	StageName     string `json:"stageName"`
	StagePosition int    `json:"stagePosition"`
	CourseSlug    string `json:"courseSlug"`
	RepoID        string `json:"repoId"`
	Language      string `json:"language"`
	DurationMs    *int   `json:"durationMs"`
	Logs          string `json:"logs"`
	CreatedAt     string `json:"createdAt"`
}

func (c *Client) GetSubmissionStatus(id string) (*SubmissionStatusResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/cli/submissions/"+id, nil)
	if err != nil {
		return nil, err
	}
	var resp SubmissionStatusResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func IsTerminalStatus(status string) bool {
	switch status {
	case "success", "failure", "error", "timeout":
		return true
	}
	return false
}
