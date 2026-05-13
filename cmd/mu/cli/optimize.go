package cli

import (
	"github.com/huaquanghan/mu/internal/optimize"
	"github.com/spf13/cobra"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Run system maintenance: apt autoremove, journal vacuum, cache refresh",
	Long: `Runs the following maintenance tasks:
  1. apt update && apt autoremove --purge
  2. journalctl --vacuum-size=500M
  3. update-mime-database, fc-cache

Use --dry-run to see what would run without executing.
Use --skip to exclude specific steps (e.g. --skip=apt,journal).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOptimize()
	},
}

var optimizeSkip []string

func init() {
	optimizeCmd.Flags().StringSliceVar(&optimizeSkip, "skip", nil, "Steps to skip (apt, journal, caches)")
}

func runOptimize() error {
	return optimize.Run(optimize.Options{
		DryRun: dryRun,
		Debug:  debug,
		Skip:   optimizeSkip,
	})
}
