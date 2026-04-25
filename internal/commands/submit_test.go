package commands

import (
	"strings"
	"testing"

	"github.com/bootcraft-cn/cli/internal/client"
)

func TestRenderResult_BusyLogs(t *testing.T) {
	result := &client.SubmissionStatusResponse{
		Status:    "error",
		StageSlug: "s01",
		StageName: "Hello World",
		Logs:      "[EVAL_SYSTEM_BUSY] 评测系统繁忙，请30秒后再试",
	}
	err := renderResult(result, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "繁忙") {
		t.Errorf("error message = %q, want to contain '繁忙'", err.Error())
	}
}

func TestRenderResult_GenericError(t *testing.T) {
	result := &client.SubmissionStatusResponse{
		Status:    "error",
		StageSlug: "s01",
		StageName: "Hello World",
		Logs:      "some internal error",
	}
	err := renderResult(result, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "繁忙") {
		t.Errorf("generic error should not mention '繁忙', got: %q", err.Error())
	}
}

func TestRenderResult_ErrorNoLogs(t *testing.T) {
	result := &client.SubmissionStatusResponse{
		Status: "error",
	}
	err := renderResult(result, true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "繁忙") {
		t.Errorf("empty logs should not trigger busy path, got: %q", err.Error())
	}
}
