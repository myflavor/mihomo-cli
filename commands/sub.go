package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var subCmd = &cobra.Command{
	Use:   "sub",
	Short: "Update subscription",
	RunE:  runSub,
}

func runSub(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.SubURL == "" {
		return fmt.Errorf("subUrl is not set in config.json")
	}

	fmt.Println("Fetching subscription from", cfg.SubURL)
	resp, err := http.Get(cfg.SubURL)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	subData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read subscription: %w", err)
	}

	// Parse subscription as YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(subData, &config); err != nil {
		return fmt.Errorf("failed to parse subscription YAML: %w", err)
	}

	cliDir := getCliDir()
	mihomoDir := filepath.Join(cliDir, "mihomo")
	basePath := filepath.Join(cliDir, cfg.BasePath)
	overridePath := filepath.Join(cliDir, cfg.OverridePath)
	configPath := filepath.Join(mihomoDir, "config.yaml")

	// If base exists, merge first (base -> subscription)
	if cfg.BasePath != "" && fileExists(basePath) {
		fmt.Println("Merging with base:", basePath)
		baseData, err := os.ReadFile(basePath)
		if err != nil {
			return fmt.Errorf("failed to read base: %w", err)
		}

		var baseConfig map[string]interface{}
		if err := yaml.Unmarshal(baseData, &baseConfig); err != nil {
			return fmt.Errorf("failed to parse base YAML: %w", err)
		}

		config = mergeYAML(baseConfig, config)
	}

	// If override exists, merge last (subscription -> override)
	if cfg.OverridePath != "" && fileExists(overridePath) {
		fmt.Println("Merging with override:", overridePath)
		overrideData, err := os.ReadFile(overridePath)
		if err != nil {
			return fmt.Errorf("failed to read override: %w", err)
		}

		var overrideConfig map[string]interface{}
		if err := yaml.Unmarshal(overrideData, &overrideConfig); err != nil {
			return fmt.Errorf("failed to parse override YAML: %w", err)
		}

		config = mergeYAML(config, overrideConfig)
	}

	// Write output
	outData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, outData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Config written to", configPath)

	// Restart mihomo service if running
	runSudoCmd("systemctl", "restart", "mihomo")

	return nil
}

func mergeYAML(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if m1, ok := result[k].(map[string]interface{}); ok {
			if m2, ok := v.(map[string]interface{}); ok {
				result[k] = mergeYAML(m1, m2)
				continue
			}
		}
		result[k] = v
	}
	return result
}

func loadConfig() (*Config, error) {
	cfgPath := filepath.Join(getCliDir(), "config.json")
	return LoadConfig(cfgPath)
}

func LoadConfig(path string) (*Config, error) {
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type Config struct {
	SubURL       string `json:"subUrl"`
	BasePath     string `json:"basePath"`
	OverridePath string `json:"overridePath"`
}
