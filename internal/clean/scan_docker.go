package clean

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

const dockerSocket = "/var/run/docker.sock"

// dockerDfLine represents one line from `docker system df --format '{{json .}}'`.
type dockerDfLine struct {
	Type        string `json:"Type"`
	Reclaimable string `json:"Reclaimable"`
}

func parseDockerBuildCacheSize(jsonLines string) int64 {
	for _, line := range strings.Split(jsonLines, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry dockerDfLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "Build Cache" {
			continue
		}
		return parseDockerSize(entry.Reclaimable)
	}
	return 0
}

// parseDockerSize converts Docker size strings like "1.2GB", "512MB", "0B" to bytes.
func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	unit = strings.ToUpper(unit)
	switch {
	case strings.HasPrefix(unit, "GB") || strings.HasPrefix(unit, "G"):
		return int64(val * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "MB") || strings.HasPrefix(unit, "M"):
		return int64(val * 1024 * 1024)
	case strings.HasPrefix(unit, "KB") || strings.HasPrefix(unit, "K"):
		return int64(val * 1024)
	case strings.HasPrefix(unit, "B"):
		return int64(val)
	}
	return 0
}

// newDockerTarget builds the Docker build-cache CleanTarget (always OptIn).
func newDockerTarget() CleanTarget {
	return CleanTarget{
		ID:    "docker",
		Label: "Docker Build Cache",
		OptIn: true,
		Scan: func() (int64, error) {
			result, err := cleanRunner.Run(context.Background(), "docker", "system", "df", "--format", "{{json .}}")
			if err != nil {
				return 0, err
			}
			return parseDockerBuildCacheSize(string(result.Stdout)), nil
		},
		Execute: func(dryRun bool) error {
			if dryRun {
				utils.LogOutcome("docker-builder-prune", "build cache", "dry-run")
				return nil
			}
			if _, err := cleanRunner.Run(context.Background(), "docker", "builder", "prune", "-f"); err != nil {
				return fmt.Errorf("docker builder prune: %w", err)
			}
			utils.LogOutcome("docker-builder-prune", "build cache", "success")
			return nil
		},
	}
}

// dockerTarget returns nil if the Docker socket is absent, or a CleanTarget for Docker build cache.
func dockerTarget() *CleanTarget {
	if !utils.PathExists(dockerSocket) {
		return nil
	}
	t := newDockerTarget()
	return &t
}
