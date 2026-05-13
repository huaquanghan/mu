package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

// RemoveAPT purges APT packages. If dryRun, only prints what would run.
func RemoveAPT(names []string, dryRun bool) error {
	if len(names) == 0 {
		return nil
	}
	if dryRun {
		fmt.Printf("  [dry-run] would run: sudo apt purge -y %s\n", strings.Join(names, " "))
		return nil
	}
	args := append([]string{"apt", "purge", "-y"}, names...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt purge: %w", err)
	}
	for _, n := range names {
		utils.LogOp("apt-purge", n)
	}
	return nil
}

// RemoveSnap removes snap packages one at a time.
func RemoveSnap(names []string, dryRun bool) error {
	for _, name := range names {
		if dryRun {
			fmt.Printf("  [dry-run] would run: sudo snap remove %s\n", name)
			continue
		}
		cmd := exec.Command("sudo", "snap", "remove", name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warn: snap remove %s: %v\n", name, err)
		}
		utils.LogOp("snap-remove", name)
	}
	return nil
}

// RemoveRemnants deletes remnant dirs using SafeDelete.
// Skips /var paths — they are shown for info only and require root.
func RemoveRemnants(paths []string, dryRun bool) error {
	for _, p := range paths {
		if strings.HasPrefix(p, "/var/") {
			continue
		}
		if err := utils.SafeDelete(p, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "warn: %v\n", err)
			continue
		}
		utils.LogOp("remnant-remove", p)
	}
	return nil
}
