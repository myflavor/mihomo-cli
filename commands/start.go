package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start mihomo in background",
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	cliDir := getCliDir()
	mihomoDir := filepath.Join(cliDir, "mihomo")
	mihomoPath := filepath.Join(mihomoDir, "mihomo")
	configPath := filepath.Join(mihomoDir, "config.yaml")

	if !fileExists(mihomoPath) {
		return fmt.Errorf("mihomo binary not found at %s, run 'mihomo-cli download' first", mihomoPath)
	}

	pidPath := filepath.Join(getCliDir(), ".mihomo.pid")

	// Check if already running
	if fileExists(pidPath) {
		pidData, _ := os.ReadFile(pidPath)
		if pid, err := strconv.Atoi(string(pidData)); err == nil {
			if processExists(pid) {
				return fmt.Errorf("mihomo is already running with PID %d", pid)
			}
		}
	}

	fmt.Println("Starting mihomo...")
	logPath := filepath.Join(getCliDir(), "mihomo.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	mihomoCmd := exec.Command(mihomoPath, "-d", mihomoDir, "-f", configPath)
	mihomoCmd.Stdout = logFile
	mihomoCmd.Stderr = logFile
	if err := mihomoCmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start mihomo: %w", err)
	}
	logFile.Close()
	pid := mihomoCmd.Process.Pid

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	fmt.Printf("mihomo started with PID %d\n", pid)
	fmt.Printf("Log file: %s\n", logPath)
	return nil
}

func processExists(pid int) bool {
	process, _ := os.FindProcess(pid)
	return process.Signal(syscall.Signal(0)) == nil
}
