package cli

import (
	"github.com/huaquanghan/mu/internal/clean"
	"github.com/spf13/cobra"
)

var cleanInclude []string

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Free disk space by removing caches, logs, and old packages",
	Long: `Scan and remove:
  • User cache (~/.cache)
  • APT package cache
  • Snap disabled revisions
  • Journal logs (older than 30 days)
  • Old kernel headers and images
  • Browser caches (Chrome, Firefox, VSCode)
  • Thumbnail cache
  • Docker build cache (if Docker is installed)

Use --dry-run to preview what would be removed without making changes.
Use --include=browser-cache to enable opt-in categories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClean()
	},
}

func init() {
	cleanCmd.Flags().StringSliceVar(&cleanInclude, "include", nil, "Opt-in categories (e.g. browser-cache)")
}

func runClean() error {
	return clean.Run(clean.Options{
		DryRun:  dryRun,
		Debug:   debug,
		Include: cleanInclude,
	})
}
