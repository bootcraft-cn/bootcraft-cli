package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAPIURL = "https://api.bootcraft.cn"
	configDirName = ".bootcraft"
	configFile    = "config.yml"
)

type Config struct {
	Token  string `yaml:"token,omitempty"`
	APIURL string `yaml:"api_url,omitempty"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDirName)
}

func configPath() string {
	return filepath.Join(configDir(), configFile)
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

func (c *Config) GetToken() string {
	if env := os.Getenv("BOOTCRAFT_TOKEN"); env != "" {
		return env
	}
	return c.Token
}

func (c *Config) GetAPIURL(flagURL string) string {
	if flagURL != "" {
		return flagURL
	}
	if c.APIURL != "" {
		return c.APIURL
	}
	return DefaultAPIURL
}
