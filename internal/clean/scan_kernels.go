package clean

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

// ParseKernelPackages parses dpkg-query output and returns (packages, totalBytes).
// It excludes any package containing runningKernel in its name.
func ParseKernelPackages(dpkgOutput, runningKernel string) ([]string, int64) {
	var pkgs []string
	var total int64
	lines := strings.Split(dpkgOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		pkg := strings.TrimSpace(parts[0])
		sizeStr := strings.TrimSpace(parts[1])
		if pkg == "" || sizeStr == "" {
			continue
		}
		// Never include running kernel
		if strings.Contains(pkg, runningKernel) {
			continue
		}
		sizeKB, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			continue
		}
		pkgs = append(pkgs, pkg)
		total += sizeKB * 1024
	}
	return pkgs, total
}

func runningKernel() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func dpkgKernelOutput() (string, error) {
	cmd := exec.Command("dpkg-query", "--show",
		"--showformat=${Package}\t${Installed-Size}\n",
		"linux-image-*", "linux-headers-*")
	out, err := cmd.Output()
	if err != nil {
		// dpkg-query exits non-zero if no packages match; treat output as valid
		if len(out) > 0 {
			return string(out), nil
		}
		return "", err
	}
	return string(out), nil
}

// kernelsTarget returns a CleanTarget for old kernel packages.
func kernelsTarget() CleanTarget {
	return CleanTarget{
		ID:           "kernels",
		Label:        "Old Kernel Packages",
		RequiresSudo: true,
		Scan: func() int64 {
			kernel := runningKernel()
			if kernel == "" {
				return 0
			}
			out, err := dpkgKernelOutput()
			if err != nil {
				return 0
			}
			_, total := ParseKernelPackages(out, kernel)
			return total
		},
		Execute: func(dryRun bool) error {
			kernel := runningKernel()
			if kernel == "" {
				return fmt.Errorf("could not determine running kernel")
			}
			out, err := dpkgKernelOutput()
			if err != nil {
				return nil // no old kernels
			}
			pkgs, _ := ParseKernelPackages(out, kernel)
			if len(pkgs) == 0 {
				return nil
			}
			for _, p := range pkgs {
				utils.LogOp("purge-kernel", p)
			}
			if dryRun {
				return nil
			}
			args := append([]string{"apt", "purge", "-y"}, pkgs...)
			cmd := exec.Command("sudo", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
}
