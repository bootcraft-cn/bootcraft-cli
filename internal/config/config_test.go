package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	// Override config dir for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir) // Windows uses USERPROFILE

	cfg := &Config{
		Token:  "bc_test123",
		APIURL: "https://test.example.com",
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	// Check file permissions
	info, err := os.Stat(filepath.Join(tmpDir, ".bootcraft", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "bc_test123" {
		t.Errorf("expected token bc_test123, got %q", loaded.Token)
	}
	if loaded.APIURL != "https://test.example.com" {
		t.Errorf("expected api_url https://test.example.com, got %q", loaded.APIURL)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir) // Windows uses USERPROFILE

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token, got %q", cfg.Token)
	}
}

func TestGetToken_EnvOverride(t *testing.T) {
	cfg := &Config{Token: "from_config"}
	t.Setenv("BOOTCRAFT_TOKEN", "from_env")

	got := cfg.GetToken()
	if got != "from_env" {
		t.Errorf("expected from_env, got %q", got)
	}
}

func TestGetAPIURL_Priority(t *testing.T) {
	cfg := &Config{APIURL: "https://config.example.com"}

	// Flag takes precedence
	got := cfg.GetAPIURL("https://flag.example.com")
	if got != "https://flag.example.com" {
		t.Errorf("expected flag URL, got %q", got)
	}

	// Config second
	got = cfg.GetAPIURL("")
	if got != "https://config.example.com" {
		t.Errorf("expected config URL, got %q", got)
	}

	// Default fallback
	cfg.APIURL = ""
	got = cfg.GetAPIURL("")
	if got != DefaultAPIURL {
		t.Errorf("expected default URL, got %q", got)
	}
}
