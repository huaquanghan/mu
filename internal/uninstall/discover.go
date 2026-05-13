package uninstall

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Package represents an installed package and its disk footprint.
type Package struct {
	Name          string
	Version       string
	Source        string // "apt" or "snap"
	InstalledKB   int64
	RemnantsKB    int64
	RemnantsFound []string
}

// DiscoverAPT returns installed APT packages with size info.
func DiscoverAPT() ([]Package, error) {
	out, err := exec.Command("dpkg-query", "--show",
		"--showformat=${Status}\t${Installed-Size}\t${Package}\t${Version}\n").Output()
	if err != nil {
		return nil, err
	}
	return parseAPTOutput(string(out)), nil
}

// parseAPTOutput parses dpkg-query output.
func parseAPTOutput(output string) []Package {
	var pkgs []Package
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) < 4 {
			continue
		}
		if !strings.HasPrefix(fields[0], "install ok installed") {
			continue
		}
		var sizeKB int64
		fmt.Sscanf(fields[1], "%d", &sizeKB)
		pkgs = append(pkgs, Package{
			Name:        strings.TrimSpace(fields[2]),
			Version:     strings.TrimSpace(fields[3]),
			Source:      "apt",
			InstalledKB: sizeKB,
		})
	}
	return pkgs
}

// DiscoverSnap returns installed snap packages. Returns empty list if snap not found.
func DiscoverSnap() ([]Package, error) {
	if _, err := exec.LookPath("snap"); err != nil {
		return nil, nil
	}
	out, err := exec.Command("snap", "list").Output()
	if err != nil {
		return nil, nil // snap may fail on minimal systems
	}
	return parseSnapOutput(string(out)), nil
}

// parseSnapOutput parses `snap list` output (skip header line).
func parseSnapOutput(output string) []Package {
	var pkgs []Package
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:        fields[0],
			Version:     fields[1],
			Source:      "snap",
			InstalledKB: 0, // snap size deferred
		})
	}
	return pkgs
}

// Discover returns all installed packages (APT + Snap) sorted by name.
func Discover() ([]Package, error) {
	apt, err := DiscoverAPT()
	if err != nil {
		return nil, err
	}
	snap, _ := DiscoverSnap()
	all := append(apt, snap...)
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}
