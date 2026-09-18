//! Path protection and cleanup-boundary validation — port of
//! `internal/utils/paths.go`. Every message matches the Go error text.

use std::path::{Path, PathBuf};

use crate::error::{Result, msg};
use crate::xdg;

/// `protectedPrefixes` — never touched without explicit --force.
const PROTECTED_PREFIXES: &[&str] = &[
    "/boot", "/etc", "/usr", "/lib", "/lib64", "/bin", "/sbin", "/proc", "/sys", "/dev", "/run",
];

pub fn is_protected(path: &Path) -> bool {
    let clean = xdg::clean(path);
    if clean == Path::new("/") {
        return true;
    }
    PROTECTED_PREFIXES
        .iter()
        .any(|p| clean == Path::new(p) || clean.starts_with(p))
}

/// `os.Lstat`-based existence check.
pub fn path_exists(path: &Path) -> bool {
    path.symlink_metadata().is_ok()
}

/// Equivalent of Go's `pathContains`: true when `child` is strictly inside
/// `parent` (lexically, after cleaning). Never true when equal or escaping.
pub fn path_contains(parent: &Path, child: &Path) -> bool {
    let parent = xdg::clean(parent);
    let child = xdg::clean(child);
    child != parent && child.starts_with(&parent)
}

/// Rejects roots whose contents cannot be safely treated as disposable cache
/// data. Port of `ValidateCleanupRoot`.
pub fn validate_cleanup_root(root: &Path) -> Result<()> {
    if root.as_os_str().is_empty() || !root.is_absolute() {
        return msg(format!("cleanup root must be absolute: {:?}", root));
    }
    let clean_root = xdg::clean(root);
    if clean_root == Path::new("/") || is_protected(&clean_root) {
        return msg(format!("unsafe cleanup root: {}", clean_root.display()));
    }
    match clean_root.symlink_metadata() {
        Ok(info) => {
            if info.file_type().is_symlink() {
                return msg(format!(
                    "cleanup root is a symlink: {}",
                    clean_root.display()
                ));
            }
            let resolved = std::fs::canonicalize(&clean_root).map_err(|e| {
                crate::error::Error::Msg(format!(
                    "resolve cleanup root {}: {}",
                    clean_root.display(),
                    e
                ))
            })?;
            if resolved != clean_root {
                return msg(format!(
                    "cleanup root traverses a symlink: {}",
                    clean_root.display()
                ));
            }
        }
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(e) => {
            return msg(format!(
                "inspect cleanup root {}: {}",
                clean_root.display(),
                e
            ));
        }
    }

    let home = xdg::home_dir();
    if home.as_os_str().is_empty() {
        return msg("resolve home directory: HOME is not set".to_string());
    }
    let clean_home = xdg::clean(&home);
    if clean_root == clean_home || path_contains(&clean_root, &clean_home) {
        return msg(format!(
            "cleanup root must not be home or its ancestor: {}",
            clean_root.display()
        ));
    }
    Ok(())
}

/// Ensures candidate stays inside root and rejects a top-level symlink, whose
/// target may escape the declared cleanup boundary. Port of
/// `ValidateCleanupCandidate`.
pub fn validate_cleanup_candidate(root: &Path, candidate: &Path) -> Result<()> {
    validate_cleanup_root(root)?;
    if candidate.as_os_str().is_empty() || !candidate.is_absolute() {
        return msg(format!(
            "cleanup candidate must be absolute: {:?}",
            candidate
        ));
    }
    let clean_root = xdg::clean(root);
    let clean_candidate = xdg::clean(candidate);
    if clean_candidate == clean_root || !path_contains(&clean_root, &clean_candidate) {
        return msg(format!(
            "cleanup candidate {} is outside root {}",
            clean_candidate.display(),
            clean_root.display()
        ));
    }
    let info = clean_candidate.symlink_metadata().map_err(|e| {
        crate::error::Error::Msg(format!(
            "inspect cleanup candidate {}: {}",
            clean_candidate.display(),
            e
        ))
    })?;
    if info.file_type().is_symlink() {
        return msg(format!(
            "cleanup candidate is a top-level symlink: {}",
            clean_candidate.display()
        ));
    }
    let resolved = std::fs::canonicalize(&clean_candidate).map_err(|e| {
        crate::error::Error::Msg(format!(
            "resolve cleanup candidate {}: {}",
            clean_candidate.display(),
            e
        ))
    })?;
    if resolved != clean_candidate {
        return msg(format!(
            "cleanup candidate traverses a symlink: {}",
            clean_candidate.display()
        ));
    }
    Ok(())
}

/// `filepath.Dir` equivalent — parent path; `.` for a bare relative name,
/// `/` for the root (`Path::parent` returns `""`/`None` for those edges).
pub fn dir(p: &Path) -> PathBuf {
    match p.parent() {
        Some(parent) if !parent.as_os_str().is_empty() => parent.to_path_buf(),
        _ => {
            if p.is_absolute() {
                PathBuf::from("/")
            } else {
                PathBuf::from(".")
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::os::unix::fs::symlink;

    fn tempdir(tag: &str) -> PathBuf {
        let d = std::env::temp_dir().join(format!(
            "mu-test-{}-{}-{:?}",
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

    // Port of TestIsProtected_SystemPaths.
    #[test]
    fn is_protected_system_paths() {
        let cases: &[(&str, bool)] = &[
            ("/", true),
            ("/boot", true),
            ("/boot/grub/grub.cfg", true),
            ("/etc", true),
            ("/etc/ssh/sshd_config", true),
            ("/usr", true),
            ("/usr/bin/ls", true),
            ("/proc/1/cmdline", true),
            ("/sys/class/net", true),
            ("/tmp/mu-test", false),
            ("/home/user/.cache/foo", false),
            ("/var/cache/apt", false),
            ("/run/docker.sock", true),
        ];
        for (path, want) in cases {
            assert_eq!(
                is_protected(Path::new(path)),
                *want,
                "is_protected({path:?})"
            );
        }
    }

    // Port of TestValidateCleanupRootRejectsDangerousRoots — HOME-dependent
    // assertions use the process home, exercising the same logic Go tests.
    #[test]
    fn validate_cleanup_root_rejects_dangerous_roots() {
        let home = xdg::home_dir();
        for root in [PathBuf::from("relative"), PathBuf::from("/"), home.clone()] {
            assert!(
                validate_cleanup_root(&root).is_err(),
                "expected root {root:?} to be rejected"
            );
        }
        let home_parent = dir(&home);
        assert!(
            validate_cleanup_root(&home_parent).is_err(),
            "expected home ancestor {home_parent:?} to be rejected"
        );
        let safe = home.join(".cache");
        fs::create_dir_all(&safe).unwrap();
        validate_cleanup_root(&safe).expect("safe root rejected");
    }

    // Port of TestValidateCleanupCandidateRejectsEscapeAndTopSymlink.
    #[test]
    fn validate_cleanup_candidate_rejects_escape_and_top_symlink() {
        let home = tempdir("cand");
        let root = home.join(".cache");
        fs::create_dir_all(&root).unwrap();
        let outside = home.join("outside");
        fs::write(&outside, b"x").unwrap();
        assert!(
            validate_cleanup_candidate(&root, &outside).is_err(),
            "expected outside candidate rejection"
        );
        let link = root.join("link");
        symlink(&outside, &link).unwrap();
        let err = validate_cleanup_candidate(&root, &link).unwrap_err();
        assert!(
            err.to_string().contains("symlink"),
            "expected symlink rejection, got {err}"
        );
        fs::remove_dir_all(&home).ok();
    }

    // Port of TestValidateCleanupRootAndCandidateRejectSymlinkTraversal.
    #[test]
    fn rejects_symlink_traversal() {
        let home = tempdir("trav");
        let real_root = home.join("real-cache");
        fs::create_dir_all(&real_root).unwrap();
        let linked_root = home.join(".cache");
        symlink(&real_root, &linked_root).unwrap();
        let err = validate_cleanup_root(&linked_root).unwrap_err();
        assert!(
            err.to_string().contains("symlink"),
            "expected linked root rejection, got {err}"
        );

        let root = home.join("safe-cache");
        let external = home.join("external");
        fs::create_dir_all(&external).unwrap();
        fs::create_dir_all(&root).unwrap();
        let parent_link = root.join("linked-parent");
        symlink(&external, &parent_link).unwrap();
        let candidate = parent_link.join("cache");
        fs::create_dir(&candidate).unwrap();
        let err = validate_cleanup_candidate(&root, &candidate).unwrap_err();
        assert!(
            err.to_string().contains("traverses a symlink"),
            "expected parent symlink rejection, got {err}"
        );
        fs::remove_dir_all(&home).ok();
    }

    #[test]
    fn path_contains_semantics() {
        assert!(path_contains(Path::new("/a"), Path::new("/a/b")));
        assert!(path_contains(Path::new("/a"), Path::new("/a/b/c")));
        assert!(!path_contains(Path::new("/a"), Path::new("/a")));
        assert!(!path_contains(Path::new("/a"), Path::new("/ab")));
        assert!(!path_contains(Path::new("/a/b"), Path::new("/a")));
        assert!(!path_contains(Path::new("/a"), Path::new("/a/../b")));
    }
}
