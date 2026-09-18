//! Operations log — port of `internal/utils/logger.go`.
//!
//! `~/.local/share/mu/operations.log`, appended with
//! `{rfc3339}  {action:<12}  {outcome:<8}  {target}` lines, rotated at 10 MB
//! keeping exactly one `.1` file. `MU_NO_OPLOG=1` disables everything.

use std::io::Write;
use std::path::Path;
use std::sync::Mutex;

use crate::error::Result;
use crate::xdg;

const LOG_MAX_BYTES: u64 = 10 * 1024 * 1024; // 10 MB

static LOG_FILE: Mutex<Option<std::fs::File>> = Mutex::new(None);

/// `InitLogger` — reads `MU_NO_OPLOG` and XDG data home from the environment.
pub fn init_logger() -> Result<()> {
    if std::env::var_os("MU_NO_OPLOG").is_some_and(|v| v == "1") {
        return Ok(());
    }
    init_logger_at(&xdg::data_home())
}

/// Testable core: `data_home` is the XDG data dir that would contain `mu/`.
pub fn init_logger_at(data_home: &Path) -> Result<()> {
    let dir = data_home.join("mu");
    // `os.MkdirAll(dir, 0o700)` — the mode applies to created dirs only;
    // an existing directory is left untouched (Go does not chmod it).
    if !dir.exists() {
        std::fs::create_dir_all(&dir)?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&dir, std::fs::Permissions::from_mode(0o700))?;
        }
    }
    let log_path = dir.join("operations.log");
    if let Ok(fi) = std::fs::metadata(&log_path)
        && fi.len() > LOG_MAX_BYTES
    {
        let _ = std::fs::rename(&log_path, dir.join("operations.log.1"));
    }
    // `os.OpenFile(_, O_CREATE|O_APPEND, 0o600)` — mode only on create.
    let mut opts = std::fs::OpenOptions::new();
    opts.append(true).create(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(0o600);
    }
    let f = opts.open(&log_path)?;
    *lock_log() = Some(f);
    Ok(())
}

/// `LOG_FILE.lock()` tolerant of poisoning — a panic inside one log call
/// must not turn every later `log_outcome` into a process crash.
fn lock_log() -> std::sync::MutexGuard<'static, Option<std::fs::File>> {
    LOG_FILE.lock().unwrap_or_else(|e| e.into_inner())
}

/// `LogOp` — record a successful operation.
pub fn log_op(action: &str, target: &str) {
    log_outcome(action, target, "success");
}

/// `LogOutcome` — record the final outcome of an operation. Callers must only
/// log success after the operation completes.
pub fn log_outcome(action: &str, target: &str, outcome: &str) {
    let mut guard = lock_log();
    if let Some(f) = guard.as_mut() {
        let _ = writeln!(
            f,
            "{}  {:<12}  {:<8}  {}",
            rfc3339_now(),
            action,
            outcome,
            target
        );
    }
}

/// `CloseLogger`.
pub fn close_logger() {
    *lock_log() = None;
}

/// `time.Now().Format(time.RFC3339)` — local time with UTC offset, truncated
/// to whole seconds (`Z` for UTC, `±hh:mm` otherwise) exactly like Go's
/// layout `2006-01-02T15:04:05Z07:00`.
fn rfc3339_now() -> String {
    let now = time::OffsetDateTime::now_local().unwrap_or_else(|_| time::OffsetDateTime::now_utc());
    let date = format!(
        "{:04}-{:02}-{:02}T{:02}:{:02}:{:02}",
        now.year(),
        u8::from(now.month()),
        now.day(),
        now.hour(),
        now.minute(),
        now.second()
    );
    let off = now.offset();
    if off.is_utc() {
        format!("{date}Z")
    } else {
        format!(
            "{date}{}{:02}:{:02}",
            if off.is_negative() { '-' } else { '+' },
            off.whole_hours().abs(),
            off.minutes_past_hour().abs()
        )
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::paths;
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

    // Port of TestLoggerRotationAndFormattingHelpers (logger part).
    #[test]
    fn logger_rotation_and_formatting() {
        let root = tempdir("oplog");
        let log_dir = root.join("mu");
        fs::create_dir_all(&log_dir).unwrap();
        let log_path = log_dir.join("operations.log");
        let large = fs::File::create(&log_path).unwrap();
        large.set_len(LOG_MAX_BYTES + 1).unwrap();
        drop(large);

        init_logger_at(&root).unwrap();
        log_op("test", "target");
        close_logger();

        assert!(
            paths::path_exists(&log_dir.join("operations.log.1")),
            "expected rotated operation log"
        );
        let data = fs::read_to_string(&log_path).unwrap();
        assert!(data.contains("success"), "log data={data:?}");
        fs::remove_dir_all(&root).ok();
    }

    #[test]
    fn rfc3339_format_shape() {
        let s = rfc3339_now();
        // e.g. 2026-09-15T07:31:45+07:00 or Z
        assert!(s.len() >= 20, "unexpected stamp {s:?}");
        assert!(s.contains('T'));
    }
}
