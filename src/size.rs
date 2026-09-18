//! Size helpers — port of `internal/utils/size.go`: `DirSize`, `HumanSize`,
//! `HumanKB`. Formats are byte-compatible with the Go output.

use std::path::Path;

use crate::error::Error;

/// `DirSize` — Go signature `(total int64, err error)`: returns the PARTIAL
/// total alongside the error (callers do `size, _ := DirSize(dir)` and keep
/// it). `DirEntry::metadata` is lstat-based, so symlinks count their own
/// size exactly as `d.Info()` does in Go.
pub fn dir_size(path: &Path) -> (u64, Option<Error>) {
    let mut total: u64 = 0;
    let mut first_err: Option<Error> = None;
    let mut stack = vec![path.to_path_buf()];
    while let Some(dir) = stack.pop() {
        match std::fs::read_dir(&dir) {
            Ok(entries) => {
                for entry in entries {
                    match entry {
                        Ok(e) => {
                            let ft = match e.file_type() {
                                Ok(ft) => ft,
                                Err(err) => {
                                    if first_err.is_none() {
                                        first_err = Some(err.into());
                                    }
                                    continue;
                                }
                            };
                            if ft.is_dir() {
                                stack.push(e.path());
                            } else {
                                match e.metadata() {
                                    Ok(m) => total += m.len(),
                                    Err(err) => {
                                        if first_err.is_none() {
                                            first_err = Some(err.into());
                                        }
                                    }
                                }
                            }
                        }
                        Err(err) => {
                            if first_err.is_none() {
                                first_err = Some(err.into());
                            }
                        }
                    }
                }
            }
            Err(err) => {
                if first_err.is_none() {
                    first_err = Some(err.into());
                }
            }
        }
    }
    (total, first_err)
}

/// `HumanSize` — `<unit` bytes print as `N B`, else `X.Y {K,M,G,T,P,E}B`.
pub fn human_size(bytes: i64) -> String {
    const UNIT: i64 = 1024;
    if bytes < UNIT {
        return format!("{bytes} B");
    }
    let mut div: i64 = UNIT;
    let mut exp = 0usize;
    let mut n = bytes / UNIT;
    while n >= UNIT {
        div *= UNIT;
        exp += 1;
        n /= UNIT;
    }
    const UNITS: &[char] = &['K', 'M', 'G', 'T', 'P', 'E'];
    format!("{:.1} {}B", bytes as f64 / div as f64, UNITS[exp])
}

/// `HumanKB` — a kilobyte count as `N KB` / `X.Y MB` / `X.Y GB`.
pub fn human_kb(kb: u64) -> String {
    const UNIT: u64 = 1024;
    if kb < UNIT {
        return format!("{kb} KB");
    }
    let mb = kb as f64 / UNIT as f64;
    if mb < UNIT as f64 {
        return format!("{mb:.1} MB");
    }
    format!("{:.1} GB", mb / UNIT as f64)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::path::PathBuf;

    fn tempdir(tag: &str) -> PathBuf {
        let d = std::env::temp_dir().join(format!(
            "mu-test-{}-{}-{}",
            tag,
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&d).unwrap();
        d
    }

    // Port of TestDirSizeCountsFiles.
    #[test]
    fn dir_size_counts_files() {
        let root = tempdir("dirsize");
        fs::write(root.join("a"), b"1234").unwrap();
        // A symlink contributes its own lstat size, as in Go.
        std::os::unix::fs::symlink(root.join("a"), root.join("link")).unwrap();
        let (size, err) = dir_size(&root);
        assert!(err.is_none());
        assert!(size >= 4, "size={size}");
        fs::remove_dir_all(&root).ok();
    }

    // Port of the size assertions inside TestLoggerRotationAndFormattingHelpers.
    #[test]
    fn human_size_formatting() {
        assert_eq!(human_size(1024), "1.0 KB");
        assert_eq!(human_kb(1024), "1.0 MB");
        assert_eq!(human_kb(1024 * 1024), "1.0 GB");
        // Golden-output spot checks (tests/golden/clean-dry-run.txt style).
        assert_eq!(human_size(202_498_651), "193.1 MB");
        assert_eq!(human_size(1_395_864_371), "1.3 GB");
        assert_eq!(human_size(2_738_284), "2.6 MB");
        assert_eq!(human_size(512), "512 B");
        assert_eq!(human_size(0), "0 B");
    }
}
