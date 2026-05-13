package cli

import (
	"github.com/huaquanghan/mu/internal/status"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Live system dashboard: CPU, RAM, disk, network, health score",
	Long: `Displays a real-time system health dashboard.

  • CPU usage (per-core and aggregate)
  • RAM and swap usage
  • Disk usage for mounted filesystems
  • Network I/O (bytes in/out)
  • Computed health score (0-100)

Outputs structured JSON when stdout is not a terminal (piped).
Press q or Ctrl+C to exit the dashboard.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func runStatus() error {
	return status.Run(status.Options{
		Debug: debug,
	})
}
