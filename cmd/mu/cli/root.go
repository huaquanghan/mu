package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	dryRun bool
	debug  bool
)

var rootCmd = &cobra.Command{
	Use:   "mu",
	Short: "Deep clean & optimize tool for Ubuntu",
	Long: `mu (Mole Ubuntu) — safe, fast system cleaner and optimizer.

Run without arguments to open the interactive TUI menu.
All destructive commands support --dry-run for safe previewing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview actions without making changes")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable verbose debug logging")

	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(statusCmd)
}
