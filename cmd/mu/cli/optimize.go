package cli

import (
	"github.com/huaquanghan/mu/internal/optimize"
	"github.com/spf13/cobra"
)

var (
	optimizeSkip   []string
	optimizeDryRun bool
	optimizeYes    bool
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Run system maintenance: apt autoremove, journal vacuum, cache refresh",
	Long: `Runs the following maintenance tasks:
  1. apt-get update && apt-get autoremove --purge
  2. journalctl --vacuum-size=500M
  3. update-mime-database, fc-cache

Use --dry-run to see what would run without executing.
Use --skip to exclude specific steps (e.g. --skip=apt,journal).
Use --yes to skip the confirmation prompt (for scripting).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := runOptimize()
		return err
	},
}

func init() {
	optimizeCmd.Flags().StringSliceVar(&optimizeSkip, "skip", nil, "Steps to skip (apt, journal, caches)")
	optimizeCmd.Flags().BoolVar(&optimizeDryRun, "dry-run", false, "Preview actions without making changes")
	optimizeCmd.Flags().BoolVarP(&optimizeYes, "yes", "y", false, "Skip confirmation prompt")
}

func runOptimize() (string, error) {
	return optimize.Run(optimize.Options{
		DryRun:  optimizeDryRun,
		Debug:   debug,
		Skip:    optimizeSkip,
		AutoYes: optimizeYes,
	})
}
