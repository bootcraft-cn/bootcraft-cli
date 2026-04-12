package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

type SubmitParams struct {
	Course        string
	Language      string
	Stage         string
	Archive       io.Reader
	CommitSHA     string
	CommitMessage string
}

type SubmitResponse struct {
	SubmissionID string `json:"submissionId"`
	Status       string `json:"status"`
	StageSlug    string `json:"stageSlug"`
	StageName    string `json:"stageName"`
}

func (c *Client) Submit(params SubmitParams) (*SubmitResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	_ = w.WriteField("course", params.Course)
	_ = w.WriteField("language", params.Language)
	if params.Stage != "" {
		_ = w.WriteField("stage", params.Stage)
	}
	if params.CommitSHA != "" {
		_ = w.WriteField("commit_sha", params.CommitSHA)
	}
	if params.CommitMessage != "" {
		_ = w.WriteField("commit_message", params.CommitMessage)
	}

	part, err := w.CreateFormFile("code", "code.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, params.Archive); err != nil {
		return nil, fmt.Errorf("writing archive: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/v1/cli/submit", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	var resp SubmitResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
