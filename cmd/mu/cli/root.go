package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -X ldflags.
var Version = "dev"

var debug bool

var rootCmd = &cobra.Command{
	Use:     "mu",
	Short:   "Deep clean & optimize tool for Ubuntu",
	Version: Version,
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
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable verbose debug logging")

	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(statusCmd)
}
