package uninstall

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/huaquanghan/mu/internal/command"
)

var uninstallRunner = command.Runner(command.ExecRunner{})

// Package represents an installed package and its disk footprint.
type Package struct {
	Name          string
	Version       string
	Source        string // "apt" or "snap"
	InstalledKB   int64
	RemnantsKB    int64
	RemnantsFound []string
}

func (p Package) Key() string { return p.Source + ":" + p.Name }

// DiscoverAPT returns installed APT packages with size info.
func DiscoverAPT() ([]Package, error) {
	result, err := uninstallRunner.Run(context.Background(), "dpkg-query", "--show",
		"--showformat=${Status}\t${Installed-Size}\t${Package}\t${Version}\n")
	if err != nil {
		return nil, err
	}
	return parseAPTOutput(string(result.Stdout)), nil
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
	if _, err := uninstallRunner.LookPath("snap"); err != nil {
		return nil, nil
	}
	result, err := uninstallRunner.Run(context.Background(), "snap", "list")
	if err != nil {
		return nil, fmt.Errorf("snap list: %w", err)
	}
	return parseSnapOutput(string(result.Stdout)), nil
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
			Name:    fields[0],
			Version: fields[1],
			Source:  "snap",
		})
	}
	populateSnapSizes(pkgs)
	return pkgs
}

func populateSnapSizes(pkgs []Package) {
	var wg sync.WaitGroup
	for i := range pkgs {
		wg.Add(1)
		go func(p *Package) {
			defer wg.Done()
			p.InstalledKB = snapInstalledKB(p.Name)
		}(&pkgs[i])
	}
	wg.Wait()
}

// snapInstalledKB returns the on-disk size of a snap's current revision in KB.
func snapInstalledKB(name string) int64 {
	result, err := uninstallRunner.Run(context.Background(), "du", "-sk", fmt.Sprintf("/snap/%s/current", name))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(result.Stdout))
	if len(fields) == 0 {
		return 0
	}
	var kb int64
	fmt.Sscan(fields[0], &kb)
	return kb
}

// Discover returns all installed packages (APT + Snap) sorted by name.
func Discover() ([]Package, error) {
	apt, err := DiscoverAPT()
	if err != nil {
		return nil, err
	}
	snap, err := DiscoverSnap()
	if err != nil {
		return nil, err
	}
	all := append(apt, snap...)
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}
