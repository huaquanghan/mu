//! Build script — emits `MU_VERSION` from `git describe` to mirror the Go
//! Makefile's `-X …/cli.Version=$(VERSION)` ldflag. Falls back to `dev` when
//! git is unavailable or the tree has no tags, matching the Go default.
use std::process::Command;

fn main() {
    // Re-run when HEAD or the current ref changes so tag transitions surface.
    // packed-refs covers `git pack-refs` runs where refs/heads may not exist.
    println!("cargo:rerun-if-changed=.git/HEAD");
    println!("cargo:rerun-if-changed=.git/refs");
    println!("cargo:rerun-if-changed=.git/packed-refs");
    // `--dirty` reflects uncommitted working-tree changes; watch src/ so
    // incremental rebuilds recompute the version after edits (mirrors the
    // Makefile's fresh `$(shell git describe …)` on every `make build`).
    println!("cargo:rerun-if-changed=src/");
    let version = git_describe().unwrap_or_else(|| "dev".to_string());
    println!("cargo:rustc-env=MU_VERSION={version}");
}

/// `git describe --tags --always --dirty 2>/dev/null || echo dev` — the
/// exact invocation from the Makefile's `VERSION` expansion.
fn git_describe() -> Option<String> {
    let out = Command::new("git")
        .args(["describe", "--tags", "--always", "--dirty"])
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let v = String::from_utf8(out.stdout).ok()?;
    let v = v.trim();
    if v.is_empty() {
        None
    } else {
        Some(v.to_string())
    }
}
