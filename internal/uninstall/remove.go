package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

// RemovalResult records package and remnant outcomes independently.
type RemovalResult struct {
	Package          Package
	Removed          bool
	DryRun           bool
	RemnantsRemoved  []string
	RemnantsRetained []string
	Err              error
}

// RemoveSelected removes each source-qualified package independently, then
// removes only remnants no remaining installed package owns.
func RemoveSelected(selected, installed []Package, dryRun bool) []RemovalResult {
	results := make([]RemovalResult, len(selected))
	removed := make(map[string]bool, len(selected))
	for i, pkg := range selected {
		results[i] = RemovalResult{Package: pkg, DryRun: dryRun}
		err := removePackage(pkg, dryRun)
		if err != nil {
			results[i].Err = err
			utils.LogOutcome(pkg.Source+"-remove", pkg.Name, "failure")
			continue
		}
		results[i].Removed = true
		removed[pkg.Key()] = true
		outcome := "success"
		if dryRun {
			outcome = "dry-run"
		}
		utils.LogOutcome(pkg.Source+"-remove", pkg.Name, outcome)
	}

	processedRemnants := map[string]bool{}
	for i := range results {
		result := &results[i]
		if !result.Removed {
			result.RemnantsRetained = append(result.RemnantsRetained, result.Package.RemnantsFound...)
			continue
		}
		for _, remnant := range result.Package.RemnantsFound {
			if processedRemnants[remnant] {
				continue
			}
			if strings.HasPrefix(filepath.Clean(remnant), "/var/") || remnantOwnedByRemaining(remnant, result.Package, installed, removed) {
				result.RemnantsRetained = append(result.RemnantsRetained, remnant)
				continue
			}
			processedRemnants[remnant] = true
			root, err := remnantRoot(remnant)
			if err == nil {
				err = utils.ValidateCleanupCandidate(root, remnant)
			}
			if err == nil {
				err = utils.SafeDelete(remnant, dryRun)
			}
			if err != nil {
				result.Err = errors.Join(result.Err, fmt.Errorf("remove remnant %s: %w", remnant, err))
				result.RemnantsRetained = append(result.RemnantsRetained, remnant)
				utils.LogOutcome("remnant-remove", remnant, "failure")
				continue
			}
			result.RemnantsRemoved = append(result.RemnantsRemoved, remnant)
			outcome := "success"
			if dryRun {
				outcome = "dry-run"
			}
			utils.LogOutcome("remnant-remove", remnant, outcome)
		}
	}
	return results
}

func removePackage(pkg Package, dryRun bool) error {
	var args []string
	switch pkg.Source {
	case "apt":
		args = []string{"apt-get", "purge", "-y", pkg.Name}
	case "snap":
		args = []string{"snap", "remove", pkg.Name}
	default:
		return fmt.Errorf("unsupported package source %q for %s", pkg.Source, pkg.Name)
	}
	if dryRun {
		fmt.Printf("  [dry-run] would run: sudo %s\n", strings.Join(args, " "))
		return nil
	}
	result, err := uninstallRunner.Run(context.Background(), "sudo", args...)
	if len(result.Stdout) > 0 {
		_, _ = os.Stdout.Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = os.Stderr.Write(result.Stderr)
	}
	if err != nil {
		return fmt.Errorf("%s remove %s: %w", pkg.Source, pkg.Name, err)
	}
	return nil
}

func remnantOwnedByRemaining(path string, owner Package, installed []Package, removed map[string]bool) bool {
	ownerIdentity := appIdentity(owner.Name)
	for _, candidate := range installed {
		if candidate.Key() == owner.Key() || removed[candidate.Key()] {
			continue
		}
		if appIdentity(candidate.Name) == ownerIdentity {
			return true
		}
		for _, candidatePath := range candidate.RemnantsFound {
			if filepath.Clean(candidatePath) == filepath.Clean(path) {
				return true
			}
		}
	}
	return false
}

func appIdentity(name string) string {
	if alias, ok := knownAliases[name]; ok {
		return strings.ToLower(alias)
	}
	return strings.ToLower(name)
}

func remnantRoot(path string) (string, error) {
	clean := filepath.Clean(path)
	for _, root := range []string{utils.XDGConfigHome(), utils.XDGDataHome(), utils.XDGCacheHome()} {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return root, nil
		}
	}
	return "", fmt.Errorf("remnant is outside managed user roots: %s", clean)
}

// RemovalErrors aggregates all failed package or remnant outcomes.
func RemovalErrors(results []RemovalResult) error {
	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", result.Package.Key(), result.Err))
		}
	}
	return errors.Join(errs...)
}
