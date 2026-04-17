package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	SubURL   string `json:"subUrl"`
	Template string `json:"template"`
	MihomoPath string `json:"mihomoPath"`
	ConfigPath string `json:"configPath"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
