package cli

import (
	"errors"
	"fmt"
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

func init() {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		code := 1
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(code)
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
