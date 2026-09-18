//! Whitelist matching — port of `internal/utils/whitelist.go` `IsWhitelisted`,
//! `MatchCacheSkip`, `ShouldSkipCacheTopLevel`, and `cachePathMatches`.

use std::path::Path;

use crate::config::{Whitelist, glob_match};
use crate::paths;
use crate::xdg;

/// `IsWhitelisted` — true if path is protected by the system or user
/// whitelist. Note the asymmetric check: a configured path protects both
/// itself/descendants AND its ancestors (pathContains runs both directions).
pub fn is_whitelisted(path: &Path, wl: &Whitelist) -> bool {
    if paths::is_protected(path) {
        return true;
    }
    let clean = xdg::clean(path);
    for p in &wl.protected_paths.system {
        let pp = xdg::clean(Path::new(p));
        if clean == pp
            || pp != Path::new("/")
                && (paths::path_contains(&pp, &clean) || paths::path_contains(&clean, &pp))
        {
            return true;
        }
    }
    false
}

/// `MatchCacheSkip` — whether `path` under `cache_home` matches any
/// `cache_skip` pattern. Patterns are relative to cache_home
/// (e.g. "go-build", "mozilla/firefox/*/startupCache").
pub fn match_cache_skip(path: &Path, cache_home: &Path, patterns: &[String]) -> bool {
    if patterns.is_empty() {
        return false;
    }
    let clean_path = xdg::clean(path);
    let clean_home = xdg::clean(cache_home);
    if clean_path == clean_home {
        return false;
    }
    if !paths::path_contains(&clean_home, &clean_path) {
        return false;
    }
    let rel = clean_path.strip_prefix(&clean_home).unwrap();
    // Go runs filepath.ToSlash here — a no-op on Unix, so rel is used as-is.
    let rel_slash = rel.to_string_lossy().into_owned();

    for pat in patterns {
        let pat = pat.trim();
        if pat.is_empty() || pat == "." {
            continue;
        }
        if cache_path_matches(&rel_slash, pat) {
            return true;
        }
    }
    false
}

/// `cachePathMatches` — rel matches pattern, or is under a non-glob prefix
/// that fully covers a directory, or has an ancestor that matches.
fn cache_path_matches(rel: &str, pattern: &str) -> bool {
    // Exact or under a plain (non-glob) prefix.
    if !pattern.contains(['*', '?', '[']) {
        return rel == pattern || rel.starts_with(&format!("{pattern}/"));
    }

    // Glob pattern: match full relative path.
    if glob_match(pattern, rel).unwrap_or(false) {
        return true;
    }

    // Segment-wise match when lengths equal.
    let rel_parts: Vec<&str> = rel.split('/').collect();
    let pat_parts: Vec<&str> = pattern.split('/').collect();
    if pat_parts.len() == rel_parts.len()
        && pat_parts
            .iter()
            .zip(rel_parts.iter())
            .all(|(pp, rp)| glob_match(pp, rp).unwrap_or(false))
    {
        return true;
    }

    // Ancestor of rel matches pattern (e.g. pattern matches a parent dir).
    let mut cur = rel.to_string();
    loop {
        let parent = {
            let p = Path::new(&cur);
            match p.parent() {
                Some(pp) if pp != Path::new("") && pp != Path::new(&cur) => {
                    pp.to_string_lossy().into_owned()
                }
                _ => break,
            }
        };
        if glob_match(pattern, &parent).unwrap_or(false) {
            return true;
        }
        cur = parent;
    }
    false
}

/// `ShouldSkipCacheTopLevel` — whether a top-level cache entry name should be
/// left entirely alone (exact denylist name or first segment of a pattern).
pub fn should_skip_cache_top_level(name: &str, patterns: &[String]) -> bool {
    let name = Path::new(name)
        .file_name()
        .map(|f| f.to_string_lossy().to_string())
        .unwrap_or_default();
    for pat in patterns {
        let pat = pat.trim();
        if pat.is_empty() {
            continue;
        }
        let first = pat.split('/').next().unwrap_or("");
        if first == name {
            return true;
        }
        if first.contains(['*', '?', '[']) && glob_match(first, &name).unwrap_or(false) {
            return true;
        }
    }
    false
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::default_whitelist;

    // Port of TestIsWhitelisted_ProtectedPaths.
    #[test]
    fn is_whitelisted_protected_paths() {
        let wl = default_whitelist();
        for p in ["/", "/etc", "/etc/passwd", "/boot", "/usr/bin/env"] {
            assert!(
                is_whitelisted(Path::new(p), &wl),
                "expected {p} to be whitelisted (protected)"
            );
        }
    }

    // Port of TestIsWhitelisted_SafePaths.
    #[test]
    fn is_whitelisted_safe_paths() {
        let wl = default_whitelist();
        for p in ["/tmp/foo", "/home/user/.cache/something", "/var/cache/apt"] {
            assert!(
                !is_whitelisted(Path::new(p), &wl),
                "expected {p} to NOT be whitelisted"
            );
        }
    }

    // Port of TestIsWhitelistedRejectsProtectedPathOverlap — a protected entry
    // also shields its ancestors.
    #[test]
    fn is_whitelisted_rejects_protected_path_overlap() {
        let mut wl = default_whitelist();
        wl.protected_paths
            .system
            .push("/home/user/.cache/app/keep".to_string());
        for path in [
            "/home/user/.cache/app/keep",
            "/home/user/.cache/app/keep/data",
            "/home/user/.cache/app",
        ] {
            assert!(
                is_whitelisted(Path::new(path), &wl),
                "expected overlapping path {path} to be protected"
            );
        }
    }

    // Port of TestMatchCacheSkip_TopLevel.
    #[test]
    fn match_cache_skip_top_level() {
        let home = Path::new("/home/u/.cache");
        let patterns = vec!["go-build".to_string(), "pip".to_string()];
        assert!(match_cache_skip(&home.join("go-build"), home, &patterns));
        assert!(match_cache_skip(
            &home.join("go-build").join("x").join("y"),
            home,
            &patterns
        ));
        assert!(!match_cache_skip(&home.join("thumbnails"), home, &patterns));
    }

    // Port of TestMatchCacheSkip_Glob.
    #[test]
    fn match_cache_skip_glob() {
        let home = Path::new("/home/u/.cache");
        let patterns = vec!["mozilla/firefox/*/startupCache".to_string()];
        assert!(match_cache_skip(
            &home
                .join("mozilla")
                .join("firefox")
                .join("abc.default")
                .join("startupCache"),
            home,
            &patterns
        ));
        assert!(!match_cache_skip(
            &home
                .join("mozilla")
                .join("firefox")
                .join("abc.default")
                .join("cache2"),
            home,
            &patterns
        ));
    }

    // Port of TestShouldSkipCacheTopLevel.
    #[test]
    fn should_skip_cache_top_level_cases() {
        let patterns = vec![
            "go-build".to_string(),
            "mozilla/firefox/*/startupCache".to_string(),
            "pip".to_string(),
        ];
        assert!(should_skip_cache_top_level("go-build", &patterns));
        assert!(should_skip_cache_top_level("mozilla", &patterns));
        assert!(!should_skip_cache_top_level("thumbnails", &patterns));
    }

    #[test]
    fn match_cache_skip_rejects_outside_paths() {
        let home = Path::new("/home/u/.cache");
        let patterns = vec!["x".to_string()];
        assert!(!match_cache_skip(home, home, &patterns)); // rel == "."
        assert!(!match_cache_skip(Path::new("/other/x"), home, &patterns));
    }
}
