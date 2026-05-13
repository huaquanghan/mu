package cli

import (
	"github.com/huaquanghan/mu/internal/uninstall"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove apps and all their config, cache, and data remnants",
	Long: `Interactively select installed packages to remove.
Shows estimated disk usage per package including remnants in:
  ~/.config/<app>  ~/.local/share/<app>  ~/.cache/<app>

Executes: apt purge <pkg> + remnant cleanup.
Use --dry-run to preview without removing anything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUninstall()
	},
}

func runUninstall() error {
	return uninstall.Run(uninstall.Options{
		DryRun: dryRun,
		Debug:  debug,
	})
}
