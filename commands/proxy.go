package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/spf13/cobra"
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

func runProxyList(cmd *cobra.Command, args []string) error {
	resp, err := http.Get("http://localhost:9090/proxies")
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
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
	apiURL := fmt.Sprintf("http://localhost:9090/proxies/%s", name)
	resp, err := http.Get(apiURL)
	if err != nil {
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

	delayURL := fmt.Sprintf("http://localhost:9090/proxies/%s/delay?timeout=5000&url=%s", name, testUrl)
	req, _ := http.NewRequest("GET", delayURL, nil)
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

	body := map[string]interface{}{"name": proxyName}
	jsonData, _ := json.Marshal(body)

	url := fmt.Sprintf("http://localhost:9090/proxies/%s", groupName)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	fmt.Printf("Set %s to %s\n", groupName, proxyName)
	return nil
}