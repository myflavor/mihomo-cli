package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var proxyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all proxies and groups via API",
	RunE:  runProxyList,
}

var proxySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set selected proxy for a group via API",
	RunE:  runProxySet,
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage mihomo proxies via API",
}

func init() {
	proxyCmd.AddCommand(proxyListCmd)
	proxyCmd.AddCommand(proxySetCmd)
	rootCmd.AddCommand(proxyCmd)
}

type MihomoConfig struct {
	ExternalController string `yaml:"external-controller"`
	Secret             string `yaml:"secret"`
}

func getMihomoConfig() (*MihomoConfig, error) {
	mihomoDir := filepath.Join(getCliDir(), "mihomo")
	configPath := filepath.Join(mihomoDir, "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg MihomoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func getAPIAddr() string {
	cfg, err := getMihomoConfig()
	if err != nil {
		return "127.0.0.1:9090"
	}

	addr := cfg.ExternalController
	if addr == "" {
		return "127.0.0.1:9090"
	}

	// Replace 0.0.0.0 with 127.0.0.1
	addr = strings.ReplaceAll(addr, "0.0.0.0", "127.0.0.1")
	return addr
}

func getAPISecret() string {
	cfg, err := getMihomoConfig()
	if err != nil || cfg.Secret == "" {
		return ""
	}
	return cfg.Secret
}

func apiRequest(method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("http://%s%s", getAPIAddr(), path)

	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(data)
	}
	req, _ := http.NewRequest(method, url, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if secret := getAPISecret(); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func runProxyList(cmd *cobra.Command, args []string) error {
	resp, err := apiRequest("GET", "/proxies", nil)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	proxies, ok := result["proxies"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no proxies data in response")
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(proxies))
	for k := range proxies {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		info := proxies[name].(map[string]interface{})

		// Only show groups (have "all" field) and skip built-in
		if name == "DIRECT" || name == "REJECT" || name == "GLOBAL" || name == "PASS" {
			continue
		}

		allProxies, isGroup := info["all"].([]interface{})
		if !isGroup {
			continue
		}

		ptype := info["type"]
		now := ""
		if n, ok := info["now"].(string); ok {
			now = n
		}
		fmt.Printf("%s (%s)\n", name, ptype)

		for _, p := range allProxies {
			pName := p.(string)
			mark := ""
			if pName == now {
				mark = " *"
			}

			// Skip delay check for DIRECT and REJECT
			if pName == "DIRECT" || pName == "REJECT" {
				fmt.Printf("  - %s%s\n", pName, mark)
				continue
			}

			delay := getProxyDelay(pName)
			if delay > 0 {
				fmt.Printf("  - %s (%dms)%s\n", pName, delay, mark)
			} else {
				fmt.Printf("  - %s (--)%s\n", pName, mark)
			}
		}
	}

	return nil
}

func getProxyDelay(name string) int {
	// Get proxy info to check for custom testUrl
	resp, err := apiRequest("GET", fmt.Sprintf("/proxies/%s", name), nil)
	if err != nil || resp.StatusCode >= 400 {
		return -1
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var info map[string]interface{}
	json.Unmarshal(data, &info)

	testUrl := "https://www.gstatic.com/generate_204"
	if tu, ok := info["testUrl"].(string); ok && tu != "" {
		testUrl = tu
	}

	delayURL := fmt.Sprintf("/proxies/%s/delay?timeout=5000&url=%s", name, testUrl)
	req, _ := http.NewRequest("GET", "http://"+getAPIAddr()+delayURL, nil)
	if secret := getAPISecret(); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err = client.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return -1
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if delay, ok := result["delay"].(float64); ok {
		return int(delay)
	}
	return -1
}

func runProxySet(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: mihomo-cli proxy set <group-name> <proxy-name>")
	}

	groupName := args[0]
	proxyName := args[1]

	resp, err := apiRequest("PUT", fmt.Sprintf("/proxies/%s", groupName), map[string]interface{}{"name": proxyName})
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Set %s to %s\n", groupName, proxyName)
	return nil
}