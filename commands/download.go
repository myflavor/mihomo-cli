package commands

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download latest mihomo binary",
	RunE:  runDownload,
}

func runDownload(cmd *cobra.Command, args []string) error {
	// Get latest release from GitHub
	resp, err := http.Get("https://api.github.com/repos/MetaCubeX/mihomo/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release info: %w", err)
	}

	// Determine OS and arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	downloadURL := ""

	for _, asset := range release.Assets {
		if goos == "linux" && goarch == "amd64" {
			// Match patterns like "mihomo-linux-amd64-v1.19.23.gz" or "mihomo-linux-amd64-compatible-v1.19.23.gz"
			if strings.Contains(asset.Name, "mihomo-linux-amd64") && strings.HasSuffix(asset.Name, ".gz") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no suitable asset found for %s/%s", goos, goarch)
	}

	fmt.Printf("Downloading mihomo %s from %s...\n", release.TagName, downloadURL)

	resp, err = http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// Create mihomo directory
	mihomoDir := filepath.Join(getCliDir(), "mihomo")
	if err := os.MkdirAll(mihomoDir, 0755); err != nil {
		return fmt.Errorf("failed to create mihomo dir: %w", err)
	}

	// Decompress .gz file and write binary directly (not a tar.gz, just gzip compressed binary)
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	out, err := os.OpenFile(filepath.Join(mihomoDir, "mihomo"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create mihomo binary: %w", err)
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		return fmt.Errorf("failed to write mihomo binary: %w", err)
	}
	out.Close()

	fmt.Println("Downloaded mihomo to", filepath.Join(mihomoDir, "mihomo"))
	return nil
}

func getCliDir() string {
	exe, _ := os.Executable()
	if exe == "" {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}
