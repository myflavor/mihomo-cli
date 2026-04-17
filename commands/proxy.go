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
	"sync"
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

var (
	configCache     *MihomoConfig
	configCacheOnce sync.Once
)

func getMihomoConfig() (*MihomoConfig, error) {
	var err error
	configCacheOnce.Do(func() {
		mihomoDir := filepath.Join(getCliDir(), "mihomo")
		configPath := filepath.Join(mihomoDir, "config.yaml")

		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			err = fmt.Errorf("failed to read config: %w", readErr)
			return
		}

		var cfg MihomoConfig
		if parseErr := yaml.Unmarshal(data, &cfg); parseErr != nil {
			err = fmt.Errorf("failed to parse config: %w", parseErr)
			return
		}

		configCache = &cfg
	})
	return configCache, err
}

func getAPIAddr() string {
	cfg, err := getMihomoConfig()
	if err != nil || cfg.ExternalController == "" {
		return "127.0.0.1:9090"
	}
	addr := strings.ReplaceAll(cfg.ExternalController, "0.0.0.0", "127.0.0.1")
	return addr
}

func getAPISecret() string {
	cfg, err := getMihomoConfig()
	if err != nil || cfg == nil {
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

	// Collect groups
	type proxyGroup struct {
		name  string
		ptype string
		now   string
		items []string
	}
	var groups []proxyGroup

	for name, info := range proxies {
		if name == "DIRECT" || name == "REJECT" || name == "GLOBAL" || name == "PASS" {
			continue
		}
		infoMap := info.(map[string]interface{})
		allProxies, isGroup := infoMap["all"].([]interface{})
		if !isGroup {
			continue
		}
		now := ""
		if n, ok := infoMap["now"].(string); ok {
			now = n
		}
		items := make([]string, 0, len(allProxies))
		for _, p := range allProxies {
			items = append(items, p.(string))
		}
		groups = append(groups, proxyGroup{name, infoMap["type"].(string), now, items})
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })

	// Concurrent delay check
	type delayResult struct {
		name  string
		delay int
	}
	results := make(chan delayResult, 100)
	var wg sync.WaitGroup

	for _, g := range groups {
		for _, item := range g.items {
			if item == "DIRECT" || item == "REJECT" {
				continue
			}
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				delay := getProxyDelayWithUrl(name)
				results <- delayResult{name, delay}
			}(item)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	delayMap := make(map[string]int)
	for r := range results {
		delayMap[r.name] = r.delay
	}

	// Print results
	for _, g := range groups {
		fmt.Printf("%s (%s)\n", g.name, g.ptype)
		for _, item := range g.items {
			mark := ""
			if item == g.now {
				mark = " *"
			}
			if item == "DIRECT" || item == "REJECT" {
				fmt.Printf("  - %s%s\n", item, mark)
			} else {
				delay := delayMap[item]
				if delay > 0 {
					fmt.Printf("  - %s (%dms)%s\n", item, delay, mark)
				} else {
					fmt.Printf("  - %s (--)%s\n", item, mark)
				}
			}
		}
	}

	return nil
}

// getProxyDelayWithUrl gets delay using testUrl from proxy info in a single request
func getProxyDelayWithUrl(name string) int {
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