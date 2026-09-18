//! `scan_browser.go` — the opt-in browser/VSCode cache target.

use std::path::{Path, PathBuf};
use std::sync::Arc;

use crate::error::Error;
use crate::{paths, trash};

use super::{CleanTarget, Deps, join_errors};

/// `browserCachePaths` — all browser cache paths that exist on the system.
/// `home` is `os.UserHomeDir()`.
pub(crate) fn browser_cache_paths_in(home: &Path) -> Vec<PathBuf> {
    let mut candidates = vec![
        home.join(".config")
            .join("google-chrome")
            .join("Default")
            .join("Cache"),
        home.join(".config").join("Code").join("CachedData"),
        home.join(".config").join("Code").join("CachedExtensions"),
    ];

    // Firefox: ~/.mozilla/firefox/*/cache2 (may match multiple profiles).
    // `filepath.Glob` semantics: each `*` candidate must stat as a directory
    // (symlinks followed) and contain a child literally named `cache2`;
    // missing/unreadable levels yield no matches rather than an error.
    let firefox = home.join(".mozilla").join("firefox");
    if let Ok(entries) = std::fs::read_dir(&firefox) {
        for e in entries.flatten() {
            let profile = e.path();
            let Ok(meta) = std::fs::metadata(&profile) else {
                continue;
            };
            if !meta.is_dir() {
                continue;
            }
            if let Ok(inner) = std::fs::read_dir(&profile)
                && inner
                    .flatten()
                    .any(|ie| ie.file_name() == std::ffi::OsStr::new("cache2"))
            {
                candidates.push(profile.join("cache2"));
            }
        }
    }

    candidates
        .into_iter()
        .filter(|p| paths::path_exists(p))
        .collect()
}

/// `browserCacheTarget` — CleanTarget for browser caches (opt-in).
pub(crate) fn browser_cache_target_in(deps: &Deps) -> CleanTarget {
    let scan_home = deps.home.clone();
    let exec_home = deps.home.clone();
    let exec_cfg = deps.config_home.clone();
    let exec_runner = Arc::clone(&deps.trash_runner);
    let exec_deps = deps.trash_deps.clone();

    CleanTarget {
        id: "browser-cache",
        label: "Browser Caches (Chrome/Firefox/VSCode)",
        requires_sudo: false,
        opt_in: true,
        scan: Box::new(move || {
            let mut total: i64 = 0;
            let mut scan_errors: Vec<Error> = Vec::new();
            for p in browser_cache_paths_in(&scan_home) {
                let (sz, err) = crate::size::dir_size(&p);
                match err {
                    None => total += sz as i64,
                    Some(e) => scan_errors.push(e),
                }
            }
            // Go: `return total, errors.Join(scanErrors...)` — partial
            // total rides back with the error.
            if scan_errors.is_empty() {
                (total, None)
            } else {
                (total, Some(join_errors(&scan_errors)))
            }
        }),
        preview: None,
        execute: Box::new(move |dry_run| {
            let mut delete_errors: Vec<Error> = Vec::new();
            for p in browser_cache_paths_in(&exec_home) {
                let root = browser_cleanup_root(&exec_home, &p);
                if let Err(e) = paths::validate_cleanup_candidate(&root, &p) {
                    delete_errors.push(e);
                    continue;
                }
                if let Err(e) =
                    trash::safe_delete_with(&*exec_runner, &exec_cfg, &exec_deps, &p, dry_run)
                {
                    delete_errors.push(e);
                }
            }
            if delete_errors.is_empty() {
                Ok(())
            } else {
                Err(join_errors(&delete_errors))
            }
        }),
    }
}

/// `browserCleanupRoot` — paths under `~/.mozilla` validate against the
/// `.mozilla` root; everything else against `~/.config`.
pub(crate) fn browser_cleanup_root(home: &Path, path: &Path) -> PathBuf {
    let mozilla = home.join(".mozilla");
    if path == mozilla || path.starts_with(&mozilla) {
        mozilla
    } else {
        home.join(".config")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::clean::test_support::*;
    use crate::runner::FakeRunner;
    use std::fs;

    // scan_browser_test.go: TestBrowserCacheTarget_scan
    #[test]
    fn browser_cache_target_scan() {
        let tmp = tempdir("browserscan");
        let cache_dir = tmp
            .join(".config")
            .join("google-chrome")
            .join("Default")
            .join("Cache");
        fs::create_dir_all(&cache_dir).unwrap();
        fs::write(cache_dir.join("testfile"), "hello cache").unwrap();
        let deps = deps_for_test(&tmp, Arc::new(FakeRunner::new()));
        let target = browser_cache_target_in(&deps);
        let (sz, scan_err) = (target.scan)();
        assert!(scan_err.is_none(), "browser scan: {scan_err:?}");
        assert!(sz > 0, "expected non-zero scan size, got {sz}");
        fs::remove_dir_all(&tmp).ok();
    }

    // scan_browser_test.go: TestBrowserCacheTarget_isOptIn
    #[test]
    fn browser_cache_target_is_opt_in() {
        let tmp = tempdir("browseroptin");
        let deps = deps_for_test(&tmp, Arc::new(FakeRunner::new()));
        let target = browser_cache_target_in(&deps);
        assert!(target.opt_in, "browser cache target should be opt-in");
        fs::remove_dir_all(&tmp).ok();
    }

    // scan_browser_test.go: TestBrowserCacheTarget_scanEmpty
    #[test]
    fn browser_cache_target_scan_empty() {
        let tmp = tempdir("browserempty");
        let deps = deps_for_test(&tmp, Arc::new(FakeRunner::new()));
        let target = browser_cache_target_in(&deps);
        let (sz, scan_err) = (target.scan)();
        assert!(scan_err.is_none(), "browser scan: {scan_err:?}");
        assert_eq!(sz, 0, "expected 0 for empty home, got {sz}");
        fs::remove_dir_all(&tmp).ok();
    }

    // scan_browser_test.go: TestBrowserCacheTargetDryRunValidatesAndKeepsCache
    #[test]
    fn browser_cache_dry_run_validates_and_keeps_cache() {
        let tmp = tempdir("browserdry");
        let cache = tmp.join(".config").join("Code").join("CachedData");
        fs::create_dir_all(&cache).unwrap();
        let deps = deps_for_test(&tmp, Arc::new(FakeRunner::new()));
        let target = browser_cache_target_in(&deps);
        (target.execute)(true).expect("dry-run execute");
        assert!(
            std::fs::symlink_metadata(&cache).is_ok(),
            "dry-run removed browser cache"
        );
        assert_eq!(
            browser_cleanup_root(&tmp, &cache),
            tmp.join(".config"),
            "cleanup root"
        );
        let mozilla_cache = tmp
            .join(".mozilla")
            .join("firefox")
            .join("x")
            .join("cache2");
        assert_eq!(
            browser_cleanup_root(&tmp, &mozilla_cache),
            tmp.join(".mozilla"),
            "mozilla root"
        );
        fs::remove_dir_all(&tmp).ok();
    }
}
