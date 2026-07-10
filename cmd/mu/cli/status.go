package cli

import (
	"github.com/huaquanghan/mu/internal/status"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Live system dashboard: CPU, RAM, disk, network, health score",
	Long: `Displays a real-time system health dashboard with visual metric bars.

  • Aggregate CPU usage
  • RAM and swap usage
  • Disk usage for mounted filesystems (used % bars)
  • Network I/O rates (active interfaces)
  • Computed health score (0-100)

Use --json to output a structured JSON snapshot and exit.
When piped (stdout is not a terminal), JSON is used automatically.
Press q or Ctrl+C to exit the dashboard.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output JSON snapshot and exit")
}

func runStatus() error {
	return status.Run(status.Options{
		Debug: debug,
		JSON:  statusJSON,
	})
}
