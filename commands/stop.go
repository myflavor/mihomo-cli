package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop mihomo",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	pidPath := filepath.Join(getCliDir(), ".mihomo.pid")

	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	os.Remove(pidPath)
	fmt.Printf("mihomo (PID %d) stopped\n", pid)
	return nil
}
