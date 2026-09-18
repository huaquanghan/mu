//! Health score ported from `internal/status/health.go`.

use super::model::{DiskStat, MemStats};

/// HealthScore computes a 0–100 system health score.
/// cpu: 0–100 (current CPU usage %)
/// mem: current memory stats
/// disks: current disk stats
pub fn health_score(cpu: f64, mem: MemStats, disks: &[DiskStat]) -> i64 {
    let cpu_score = 30.0 * (1.0 - cpu / 100.0);

    let mut ram_score = 0.0;
    if mem.total_kb > 0 {
        ram_score = 30.0 * (mem.available_kb as f64 / mem.total_kb as f64);
    }

    let disk_score = 30.0 * root_disk_free_percent(disks);

    let swap_score = if mem.swap_total_kb == 0 {
        10.0
    } else {
        10.0 * (mem.swap_free_kb as f64 / mem.swap_total_kb as f64)
    };

    let total = cpu_score + ram_score + disk_score + swap_score;
    total.clamp(0.0, 100.0).round() as i64
}

fn root_disk_free_percent(disks: &[DiskStat]) -> f64 {
    for d in disks {
        if d.mount == "/" {
            if d.total_bytes == 0 {
                return 0.0;
            }
            return d.free_bytes as f64 / d.total_bytes as f64;
        }
    }
    0.0
}

/// HealthScoreAvailable avoids awarding healthy points for unavailable metrics.
pub fn health_score_available(
    cpu: f64,
    cpu_available: bool,
    mem: MemStats,
    mem_available: bool,
    disks: &[DiskStat],
) -> i64 {
    let cpu = if cpu_available { cpu } else { 100.0 };
    let mem = if mem_available {
        mem
    } else {
        MemStats {
            total_kb: 1,
            available_kb: 0,
            swap_total_kb: 1,
            swap_free_kb: 0,
        }
    };
    health_score(cpu, mem, disks)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn missing_disk_does_not_look_fully_healthy() {
        let mem = MemStats {
            total_kb: 16_000_000,
            available_kb: 16_000_000,
            swap_total_kb: 0,
            swap_free_kb: 0,
        };
        let score = health_score(0.0, mem, &[]);
        assert_eq!(score, 70, "expected 70 without a root disk metric");
    }

    #[test]
    fn max_stress() {
        let mem = MemStats {
            total_kb: 16_000_000,
            available_kb: 0,
            swap_total_kb: 4_000_000,
            swap_free_kb: 0,
        };
        let disks = [DiskStat {
            mount: "/".to_string(),
            total_bytes: 1000,
            free_bytes: 0,
        }];
        assert_eq!(health_score(100.0, mem, &disks), 0);
    }

    #[test]
    fn mid_range() {
        let mem = MemStats {
            total_kb: 16_000_000,
            available_kb: 8_000_000,
            swap_total_kb: 4_000_000,
            swap_free_kb: 2_000_000,
        };
        let disks = [DiskStat {
            mount: "/".to_string(),
            total_bytes: 1_000_000,
            free_bytes: 500_000,
        }];
        let score = health_score(50.0, mem, &disks);
        assert!(
            (40..=60).contains(&score),
            "expected mid-range score (~50), got {score}"
        );
    }

    #[test]
    fn uses_root_filesystem_only() {
        let mem = MemStats {
            total_kb: 100,
            available_kb: 100,
            ..MemStats::default()
        };
        let disks = [
            DiskStat {
                mount: "/".to_string(),
                total_bytes: 100,
                free_bytes: 100,
            },
            DiskStat {
                mount: "/sys/firmware/efi/efivars".to_string(),
                total_bytes: 100,
                free_bytes: 0,
            },
        ];
        let score = health_score(0.0, mem, &disks);
        assert_eq!(score, 100, "pseudo mount affected root health");
    }

    #[test]
    fn available_does_not_reward_missing_metrics() {
        let score = health_score_available(0.0, false, MemStats::default(), false, &[]);
        assert_eq!(score, 0, "missing metrics produced healthy score {score}");
    }
}
