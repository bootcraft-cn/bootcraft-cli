package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSON_SuccessEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": "123", "username": "test"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	var resp MeResponse
	req, _ := http.NewRequest("GET", srv.URL+"/v1/cli/me", nil)
	err := c.doJSON(req, &resp)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Username != "test" {
		t.Errorf("expected username test, got %q", resp.Username)
	}
}

func TestDoJSON_ErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   map[string]any{"code": "UNAUTHORIZED", "message": "Invalid token"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "bad-token")
	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	err := c.doJSON(req, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED, got %q", apiErr.Code)
	}
}

func TestInitCLIAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/cli-auth/init" {
			t.Errorf("expected /v1/cli-auth/init, got %s", r.URL.Path)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"code":      "bc_tmp_abc123",
			"authUrl":   "https://bootcraft.cn/cli-auth?code=bc_tmp_abc123",
			"expiresIn": 300,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	resp, err := c.InitCLIAuth()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != "bc_tmp_abc123" {
		t.Errorf("expected code bc_tmp_abc123, got %q", resp.Code)
	}
	if resp.ExpiresIn != 300 {
		t.Errorf("expected expiresIn 300, got %d", resp.ExpiresIn)
	}
}

func TestGetCLIAuthToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "valid" {
			json.NewEncoder(w).Encode(map[string]any{
				"status":   "success",
				"token":    "bc_final_token",
				"username": "william",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"status": "pending",
			})
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "")

	resp, err := c.GetCLIAuthToken("valid")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "success" || resp.Token != "bc_final_token" {
		t.Errorf("unexpected response: %+v", resp)
	}

	resp, err = c.GetCLIAuthToken("pending")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "pending" {
		t.Errorf("expected pending, got %q", resp.Status)
	}
}

func TestSubmit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/cli/submit" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			t.Error("missing content-type")
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("course") != "tinynum" {
			t.Errorf("expected course tinynum, got %q", r.FormValue("course"))
		}
		if r.FormValue("language") != "python" {
			t.Errorf("expected language python, got %q", r.FormValue("language"))
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"submissionId": "sub-123",
			"status":       "evaluating",
			"stageSlug":    "E01",
			"stageName":    "Test Stage",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")

	buf := []byte("fake-archive-content")
	resp, err := c.Submit(SubmitParams{
		Course:   "tinynum",
		Language: "python",
		Archive:  bytes.NewReader(buf),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SubmissionID != "sub-123" {
		t.Errorf("expected sub-123, got %q", resp.SubmissionID)
	}
}

func TestGetSubmissionStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":        "sub-123",
			"status":    "success",
			"stageSlug": "E01",
			"stageName": "Test Stage",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	resp, err := c.GetSubmissionStatus("sub-123")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %q", resp.Status)
	}
}

func TestIsTerminalStatus(t *testing.T) {
	if !IsTerminalStatus("success") {
		t.Error("success should be terminal")
	}
	if !IsTerminalStatus("failure") {
		t.Error("failure should be terminal")
	}
	if IsTerminalStatus("evaluating") {
		t.Error("evaluating should not be terminal")
	}
}

func TestGetTriggerToken_DeserializesStreamURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/cli/submissions/sub-abc/trigger-token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"publicAccessToken": "tok_public_xyz",
				"triggerRunId":      "run_abc123",
				"expiresAt":         "2026-04-23T12:00:00Z",
				"streamUrl":         "https://api.trigger.dev/realtime/v1/streams/run_abc123/eval-logs",
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	resp, err := c.GetTriggerToken("sub-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PublicAccessToken != "tok_public_xyz" {
		t.Errorf("PublicAccessToken: got %q, want %q", resp.PublicAccessToken, "tok_public_xyz")
	}
	if resp.TriggerRunID != "run_abc123" {
		t.Errorf("TriggerRunID: got %q, want %q", resp.TriggerRunID, "run_abc123")
	}
	if resp.StreamURL != "https://api.trigger.dev/realtime/v1/streams/run_abc123/eval-logs" {
		t.Errorf("StreamURL: got %q", resp.StreamURL)
	}
}

func TestGetTriggerToken_MissingStreamURL_BackwardCompat(t *testing.T) {
	// Old servers that don't return streamUrl should yield empty string, not an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"publicAccessToken": "tok_xyz",
				"triggerRunId":      "run_123",
				"expiresAt":         "2026-04-23T12:00:00Z",
				// streamUrl intentionally absent
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	resp, err := c.GetTriggerToken("sub-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StreamURL != "" {
		t.Errorf("StreamURL should be empty for old server response, got %q", resp.StreamURL)
	}
}
