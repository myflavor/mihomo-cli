package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install mihomo as systemd service",
	RunE:  runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall mihomo systemd service",
	RunE:  runServiceUninstall,
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage mihomo systemd service",
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	rootCmd.AddCommand(serviceCmd)
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	serviceSrc := filepath.Join(getCliDir(), "service.d", "mihomo.service")
	serviceDst := "/etc/systemd/system/mihomo.service"

	if !fileExists(serviceSrc) {
		return fmt.Errorf("service template not found at %s", serviceSrc)
	}

	// Read template and replace path
	templateData, err := os.ReadFile(serviceSrc)
	if err != nil {
		return fmt.Errorf("failed to read service template: %w", err)
	}

	cliDir := getCliDir()
	mihomoDir := filepath.Join(cliDir, "mihomo")
	mihomoBinary := filepath.Join(mihomoDir, "mihomo")
	configFile := filepath.Join(mihomoDir, "config.yaml")

	content := strings.ReplaceAll(string(templateData), "{{CLI_DIR}}", cliDir)
	content = strings.ReplaceAll(content, "{{MIHOMO_DIR}}", mihomoDir)
	content = strings.ReplaceAll(content, "{{MIHOMO_BINARY}}", mihomoBinary)
	content = strings.ReplaceAll(content, "{{CONFIG_FILE}}", configFile)

	// Write to temp file then copy with sudo
	tmpPath := "/tmp/mihomo.service"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp service file: %w", err)
	}

	// Copy to system location
	if err := runSudoCmd("cp", tmpPath, serviceDst); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	// Reload systemd
	if err := runSudoCmd("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable and start
	if err := runSudoCmd("systemctl", "enable", "mihomo"); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := runSudoCmd("systemctl", "start", "mihomo"); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("mihomo service installed and started")
	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	serviceDst := "/etc/systemd/system/mihomo.service"

	// Stop and disable (ignore errors if service not running)
	runSudoCmd("systemctl", "stop", "mihomo")
	runSudoCmd("systemctl", "disable", "mihomo")

	// Remove service file
	if err := runSudoCmd("rm", serviceDst); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	if err := runSudoCmd("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	fmt.Println("mihomo service uninstalled")
	return nil
}

func runSudoCmd(name string, args ...string) error {
	fullArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
