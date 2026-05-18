package cli

import (
	"github.com/huaquanghan/mu/internal/uninstall"
	"github.com/spf13/cobra"
)

var uninstallDryRun bool

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

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallDryRun, "dry-run", false, "Preview actions without making changes")
}

func runUninstall() error {
	return uninstall.Run(uninstall.Options{
		DryRun: uninstallDryRun,
		Debug:  debug,
	})
}
