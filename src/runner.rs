//! Ported in core-safety phase (rust-rewrite plan).
//!
//! Port of `internal/command/runner.go`: a [`Runner`] abstraction for
//! executing external commands with captured output, a [`ProcessRunner`]
//! backed by [`std::process::Command`], and a [`FakeRunner`] test double
//! (the Go callers stub the same interface via `cleanRunnerFunc` /
//! `uninstallRunnerFunc` closures to assert argv and simulate failures).
//!
//! Go's `context.Context` parameter is mirrored by an optional
//! [`Duration`] timeout on [`CommandSpec`]: `context.Background()` maps to
//! `None`, `context.WithTimeout(ctx, d)` (used by `scan_kernels.go` for the
//! 30s apt preview) maps to `Some(d)`, and an already-cancelled context maps
//! to `Duration::ZERO`. A timed-out process is killed like
//! `exec.CommandContext` (SIGKILL) and reported as [`RunError::Timeout`]
//! carrying the output captured so far.

use std::ffi::{OsStr, OsString};
use std::io::{self, Read};
use std::path::{Path, PathBuf};
use std::process::{Command, ExitStatus, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use thiserror::Error;

/// Poll granularity for the timeout path; small enough that tests using
/// ~100ms deadlines stay fast.
const POLL_INTERVAL: Duration = Duration::from_millis(5);

/// A command to execute: program plus argv, with optional extra environment
/// variables, working directory, and timeout.
///
/// `env` and `dir` have no Go counterpart on `Run` (Go callers shell out to
/// `env LC_ALL=C ...` instead); they exist so Rust callers can express the
/// same thing without a wrapper binary. Both default to inherit.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct CommandSpec {
    pub program: OsString,
    pub args: Vec<OsString>,
    /// Extra variables layered on the inherited environment.
    pub env: Vec<(OsString, OsString)>,
    pub dir: Option<PathBuf>,
    /// The `context.Context` stand-in: `None` never times out.
    pub timeout: Option<Duration>,
}

impl CommandSpec {
    pub fn new<P, A, S>(program: P, args: A) -> Self
    where
        P: AsRef<OsStr>,
        A: IntoIterator<Item = S>,
        S: AsRef<OsStr>,
    {
        Self {
            program: program.as_ref().to_os_string(),
            args: args
                .into_iter()
                .map(|arg| arg.as_ref().to_os_string())
                .collect(),
            ..Self::default()
        }
    }

    /// Adds one environment variable (on top of the inherited environment).
    pub fn env(mut self, key: impl AsRef<OsStr>, value: impl AsRef<OsStr>) -> Self {
        self.env
            .push((key.as_ref().to_os_string(), value.as_ref().to_os_string()));
        self
    }

    /// Sets the working directory.
    pub fn dir(mut self, dir: impl Into<PathBuf>) -> Self {
        self.dir = Some(dir.into());
        self
    }

    /// Sets the timeout. `Duration::ZERO` mirrors an already-cancelled
    /// context: the command is not spawned at all.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = Some(timeout);
        self
    }
}

/// Captured result of a completed command, mirroring Go's `command.Result`.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Output {
    pub stdout: Vec<u8>,
    pub stderr: Vec<u8>,
    /// Exit status, or -1 when the process was killed by a signal — the same
    /// convention as Go's `exec.ExitError.ExitCode()`.
    pub exit_code: i32,
    /// The terminating signal number when the process died by one; needed so
    /// error text can name it (`signal: terminated`, not just "killed") as
    /// Go's `ExitError.Error()` does.
    pub signal: Option<i32>,
}

impl Output {
    /// `stdout` as a lossy UTF-8 string, for callers that did
    /// `string(result.Stdout)` in Go.
    pub fn stdout_lossy(&self) -> String {
        String::from_utf8_lossy(&self.stdout).into_owned()
    }

    /// `stderr` as a lossy UTF-8 string.
    pub fn stderr_lossy(&self) -> String {
        String::from_utf8_lossy(&self.stderr).into_owned()
    }
}

/// Failure to locate or execute a command.
///
/// Go returns `(Result, error)` as a pair, so the partially captured output
/// survives a failure; here the [`Output`] travels inside the error for the
/// variants where the process ran. Use [`RunError::output`] to reach it —
/// Go callers do `fmt.Errorf("...: %w: %s", err, TrimSpace(result.Stderr))`.
#[derive(Debug, Error)]
pub enum RunError {
    /// The executable was not found (`exec.LookPath` failure or ENOENT at
    /// spawn). Message matches Go's `exec.Error` text.
    #[error("exec: {0:?}: executable file not found in $PATH")]
    NotFound(String),
    /// The command could not be started — Go's non-`ExitError` `cmd.Run`
    /// failures, which it reports with `ExitCode = -1`.
    #[error("failed to start command: {0}")]
    Spawn(#[source] io::Error),
    /// The process ran and exited unsuccessfully (non-zero status or a
    /// signal). Message mirrors Go's `ExitError`: `exit status N` or
    /// `signal: killed`.
    #[error("{}", describe_exit(.0))]
    Failed(Output),
    /// The timeout elapsed and the process was killed (SIGKILL, like
    /// `exec.CommandContext`). Carries the output captured before the kill.
    #[error("command timed out after {0:?}")]
    Timeout(Duration, Output),
}

impl RunError {
    /// Output captured before the failure, when the process ran at all.
    /// `None` for [`RunError::NotFound`] and [`RunError::Spawn`].
    pub fn output(&self) -> Option<&Output> {
        match self {
            Self::Failed(output) | Self::Timeout(_, output) => Some(output),
            Self::NotFound(_) | Self::Spawn(_) => None,
        }
    }
}

/// Mirrors `ExitError.Error()`: "exit status N", or "signal: <name>" when the
/// process was signalled — the name is Go's `Signal.String()` (lowercase
/// descriptive, e.g. "terminated", "segmentation fault").
fn describe_exit(output: &Output) -> String {
    if output.exit_code < 0 {
        format!("signal: {}", go_signal_name(output.signal))
    } else {
        format!("exit status {}", output.exit_code)
    }
}

/// Go's `syscall.Signal.String()` names (signames_linux + defaults).
fn go_signal_name(signal: Option<i32>) -> String {
    use nix::sys::signal::Signal;
    match signal.and_then(|s| Signal::try_from(s).ok()) {
        Some(Signal::SIGHUP) => "hangup",
        Some(Signal::SIGINT) => "interrupt",
        Some(Signal::SIGQUIT) => "quit",
        Some(Signal::SIGILL) => "illegal instruction",
        Some(Signal::SIGTRAP) => "trace/breakpoint trap",
        Some(Signal::SIGABRT) => "abort",
        Some(Signal::SIGBUS) => "bus error",
        Some(Signal::SIGFPE) => "floating point exception",
        Some(Signal::SIGKILL) => "killed",
        Some(Signal::SIGUSR1) => "user defined signal 1",
        Some(Signal::SIGSEGV) => "segmentation fault",
        Some(Signal::SIGUSR2) => "user defined signal 2",
        Some(Signal::SIGPIPE) => "broken pipe",
        Some(Signal::SIGALRM) => "alarm clock",
        Some(Signal::SIGTERM) => "terminated",
        Some(Signal::SIGSTKFLT) => "stack fault",
        Some(Signal::SIGCHLD) => "child exited",
        Some(Signal::SIGCONT) => "continued",
        Some(Signal::SIGSTOP) => "stopped",
        Some(Signal::SIGTSTP) => "stopped (terminal)",
        Some(Signal::SIGTTIN) => "stopped (tty input)",
        Some(Signal::SIGTTOU) => "stopped (tty output)",
        Some(Signal::SIGURG) => "urgent I/O condition",
        Some(Signal::SIGXCPU) => "CPU time limit exceeded",
        Some(Signal::SIGXFSZ) => "file size limit exceeded",
        Some(Signal::SIGVTALRM) => "virtual timer expired",
        Some(Signal::SIGPROF) => "profiling timer expired",
        Some(Signal::SIGWINCH) => "window size changed",
        Some(Signal::SIGIO) => "I/O possible",
        Some(Signal::SIGPWR) => "power fail/restart",
        Some(Signal::SIGSYS) => "bad system call",
        _ => return "unknown signal".to_string(),
    }
    .to_string()
}

/// Executes external commands. Object-safe so callers can hold
/// `Box<dyn Runner>` the way Go callers hold a `command.Runner` interface
/// value (and swap in a fake for tests).
pub trait Runner {
    /// Runs `spec` to completion, capturing stdout and stderr.
    ///
    /// A non-zero exit — or death by signal — is [`RunError::Failed`], not
    /// `Ok`, matching Go where `cmd.Run` returns `*exec.ExitError` while
    /// `Result` stays populated.
    fn run(&self, spec: &CommandSpec) -> Result<Output, RunError>;

    /// Resolves `file` against `PATH` like Go's `exec.LookPath`. Callers use
    /// this for feature detection (`snap`, `gio`, `journalctl`).
    fn look_path(&self, file: &str) -> Result<PathBuf, RunError>;
}

/// [`Runner`] backed by [`std::process::Command`] — Go's `ExecRunner`.
#[derive(Clone, Copy, Debug, Default)]
pub struct ProcessRunner;

impl ProcessRunner {
    pub fn new() -> Self {
        Self
    }
}

impl Runner for ProcessRunner {
    fn run(&self, spec: &CommandSpec) -> Result<Output, RunError> {
        let mut cmd = Command::new(&spec.program);
        cmd.args(&spec.args);
        if !spec.env.is_empty() {
            cmd.envs(spec.env.iter().map(|(key, value)| (key, value)));
        }
        if let Some(dir) = &spec.dir {
            cmd.current_dir(dir);
        }
        match spec.timeout {
            None => run_untimed(&mut cmd, spec),
            Some(timeout) => run_timed(&mut cmd, spec, timeout),
        }
    }

    fn look_path(&self, file: &str) -> Result<PathBuf, RunError> {
        look_path(file)
    }
}

/// `cmd.Run` with no deadline: `Command::output` waits and captures both
/// pipes without stalling on a full pipe buffer.
fn run_untimed(cmd: &mut Command, spec: &CommandSpec) -> Result<Output, RunError> {
    let out = cmd.output().map_err(|err| spawn_error(spec, err))?;
    output_from_status(out.status, out.stdout, out.stderr)
}

/// `Command::output` has no deadline support, so the timed path spawns the
/// child, drains both pipes on helper threads (a child that fills the pipe
/// buffer would otherwise block and never reach `try_wait`), and polls until
/// exit or the deadline. On timeout the child is killed and reaped —
/// `exec.CommandContext` sends SIGKILL too — and the partial output is kept.
fn run_timed(cmd: &mut Command, spec: &CommandSpec, timeout: Duration) -> Result<Output, RunError> {
    if timeout.is_zero() {
        // Already-cancelled context: fail without spawning.
        return Err(RunError::Timeout(timeout, Output::default()));
    }

    let mut child = cmd
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|err| spawn_error(spec, err))?;

    let mut child_stdout = child.stdout.take().expect("stdout was piped");
    let stdout_thread = thread::spawn(move || {
        let mut buf = Vec::new();
        let _ = child_stdout.read_to_end(&mut buf);
        buf
    });
    let mut child_stderr = child.stderr.take().expect("stderr was piped");
    let stderr_thread = thread::spawn(move || {
        let mut buf = Vec::new();
        let _ = child_stderr.read_to_end(&mut buf);
        buf
    });

    let deadline = Instant::now() + timeout;
    loop {
        match child.try_wait() {
            Ok(Some(status)) => {
                let stdout = stdout_thread.join().unwrap_or_default();
                let stderr = stderr_thread.join().unwrap_or_default();
                return output_from_status(status, stdout, stderr);
            }
            Ok(None) if Instant::now() < deadline => thread::sleep(POLL_INTERVAL),
            Ok(None) => {
                let _ = child.kill();
                let _ = child.wait();
                let stdout = stdout_thread.join().unwrap_or_default();
                let stderr = stderr_thread.join().unwrap_or_default();
                return Err(RunError::Timeout(
                    timeout,
                    Output {
                        stdout,
                        stderr,
                        exit_code: -1,
                        signal: Some(nix::libc::SIGKILL),
                    },
                ));
            }
            Err(err) => {
                let _ = child.kill();
                let _ = child.wait();
                let _ = stdout_thread.join();
                let _ = stderr_thread.join();
                return Err(RunError::Spawn(err));
            }
        }
    }
}

/// Builds the result for a finished process: `Ok` on exit code 0, otherwise
/// [`RunError::Failed`] with the captured output — and -1 when the process
/// died by signal, matching `exec.ExitError.ExitCode()`.
fn output_from_status(
    status: ExitStatus,
    stdout: Vec<u8>,
    stderr: Vec<u8>,
) -> Result<Output, RunError> {
    use std::os::unix::process::ExitStatusExt;
    let output = Output {
        stdout,
        stderr,
        exit_code: status.code().unwrap_or(-1),
        signal: status.signal(),
    };
    if status.success() {
        Ok(output)
    } else {
        Err(RunError::Failed(output))
    }
}

/// Spawn ENOENT maps to [`RunError::NotFound`] so it is indistinguishable
/// from a [`Runner::look_path`] miss, like Go's `exec.ErrNotFound`.
fn spawn_error(spec: &CommandSpec, err: io::Error) -> RunError {
    if err.kind() == io::ErrorKind::NotFound {
        RunError::NotFound(spec.program.to_string_lossy().into_owned())
    } else {
        RunError::Spawn(err)
    }
}

/// Mirrors `exec.LookPath` on Unix: a `file` containing a path separator is
/// tried directly (an empty `file` fails the same way — `""` is tried as a
/// relative path and is never executable); otherwise each `PATH` entry is
/// searched for a non-directory with any execute bit set. An empty `PATH`
/// element means the current directory, which `dir.join(file)` reproduces.
fn look_path(file: &str) -> Result<PathBuf, RunError> {
    if file.contains('/') || file.is_empty() {
        let path = Path::new(file);
        return if is_executable(path) {
            Ok(path.to_path_buf())
        } else {
            Err(RunError::NotFound(file.to_string()))
        };
    }
    let path_var = std::env::var_os("PATH").unwrap_or_default();
    for dir in std::env::split_paths(&path_var) {
        let candidate = dir.join(file);
        if is_executable(&candidate) {
            return Ok(candidate);
        }
    }
    Err(RunError::NotFound(file.to_string()))
}

/// Go's `findExecutable`: exists (symlinks followed), not a directory, and
/// at least one execute bit set.
#[cfg(unix)]
fn is_executable(path: &Path) -> bool {
    use std::os::unix::fs::PermissionsExt;
    path.metadata()
        .map(|meta| !meta.is_dir() && meta.permissions().mode() & 0o111 != 0)
        .unwrap_or(false)
}

#[cfg(not(unix))]
fn is_executable(path: &Path) -> bool {
    path.is_file()
}

/// Test double for [`Runner`], covering the patterns the Go suite uses for
/// `command.Runner` stubs: it records every invocation for exact-argv
/// assertions, and answers `run` from a FIFO queue first, then from an
/// optional handler (dispatch on `spec.program` the way the Go stub closures
/// dispatch on `name`), finally defaulting to `Ok` with empty output.
#[cfg(test)]
type FakeHandler = Box<dyn Fn(&CommandSpec) -> Result<Output, RunError> + Send + Sync>;

#[cfg(test)]
#[derive(Default)]
pub struct FakeRunner {
    invocations: std::sync::Mutex<Vec<CommandSpec>>,
    responses: std::sync::Mutex<std::collections::VecDeque<Result<Output, RunError>>>,
    handler: std::sync::Mutex<Option<FakeHandler>>,
    paths: std::sync::Mutex<std::collections::HashMap<String, PathBuf>>,
}

#[cfg(test)]
impl FakeRunner {
    pub fn new() -> Self {
        Self::default()
    }

    /// Enqueues a response consumed by the next `run` call.
    pub fn push_response(&self, response: Result<Output, RunError>) {
        self.responses.lock().unwrap().push_back(response);
    }

    /// Shorthand for queueing `Ok(Output { .. })` from `&str` parts.
    pub fn push_output(&self, stdout: &str, stderr: &str, exit_code: i32) {
        self.push_response(Ok(Output {
            stdout: stdout.as_bytes().to_vec(),
            stderr: stderr.as_bytes().to_vec(),
            exit_code,
            signal: None,
        }));
    }

    /// Installs a handler consulted when the queue is empty — the equivalent
    /// of the Go `func(ctx, name, args...) (Result, error)` stubs.
    pub fn set_handler<F>(&self, f: F)
    where
        F: Fn(&CommandSpec) -> Result<Output, RunError> + Send + Sync + 'static,
    {
        *self.handler.lock().unwrap() = Some(Box::new(f));
    }

    /// Makes `look_path(name)` succeed with `path`. Names without an entry
    /// fail with [`RunError::NotFound`].
    pub fn set_look_path(&self, name: &str, path: impl Into<PathBuf>) {
        self.paths
            .lock()
            .unwrap()
            .insert(name.to_string(), path.into());
    }

    /// Every recorded invocation, in order.
    pub fn invocations(&self) -> Vec<CommandSpec> {
        self.invocations.lock().unwrap().clone()
    }
}

#[cfg(test)]
impl Runner for FakeRunner {
    fn run(&self, spec: &CommandSpec) -> Result<Output, RunError> {
        self.invocations.lock().unwrap().push(spec.clone());
        if let Some(response) = self.responses.lock().unwrap().pop_front() {
            return response;
        }
        let handler = self.handler.lock().unwrap();
        if let Some(handler) = handler.as_ref() {
            return handler(spec);
        }
        Ok(Output::default())
    }

    fn look_path(&self, file: &str) -> Result<PathBuf, RunError> {
        self.paths
            .lock()
            .unwrap()
            .get(file)
            .cloned()
            .ok_or_else(|| RunError::NotFound(file.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // runner_test.go: TestExecRunnerCapturesOutputAndExitCode
    #[test]
    fn process_runner_captures_output_and_exit_code() {
        let spec = CommandSpec::new("sh", ["-c", "printf out; printf err >&2; exit 7"]);
        let err = ProcessRunner
            .run(&spec)
            .expect_err("expected command failure");
        assert_eq!(err.to_string(), "exit status 7");
        let output = err.output().expect("output should be captured");
        assert_eq!(output.stdout, b"out");
        assert_eq!(output.stderr, b"err");
        assert_eq!(output.exit_code, 7);
    }

    // runner_test.go: TestExecRunnerHonorsCancellation — a pre-cancelled
    // context maps to a zero timeout: fail without spawning.
    #[test]
    fn process_runner_honors_cancellation() {
        let spec = CommandSpec::new("sh", ["-c", "sleep 10"]).timeout(Duration::ZERO);
        let start = Instant::now();
        let err = ProcessRunner.run(&spec).expect_err("expected cancellation");
        assert!(matches!(err, RunError::Timeout(..)), "got {err:?}");
        assert!(start.elapsed() < Duration::from_secs(5));
    }

    // The WithTimeout side of ctx: a deadline kills a still-running process
    // (SIGKILL -> exit_code -1) and keeps the output captured so far.
    #[test]
    fn process_runner_kills_on_timeout() {
        // `exec` keeps `sleep` as the direct child: without it the orphaned
        // grandchild would hold the pipes open and the reader threads would
        // block until it exits (Go's `cmd.Wait` blocks the same way).
        let spec = CommandSpec::new("sh", ["-c", "printf partial; exec sleep 10"])
            .timeout(Duration::from_millis(100));
        let start = Instant::now();
        let err = ProcessRunner.run(&spec).expect_err("expected timeout");
        assert!(start.elapsed() < Duration::from_secs(5));
        match err {
            RunError::Timeout(_, output) => {
                assert_eq!(output.stdout, b"partial");
                assert_eq!(output.exit_code, -1);
            }
            other => panic!("expected timeout, got {other:?}"),
        }
    }

    #[test]
    fn process_runner_reports_missing_program() {
        let spec = CommandSpec::new("mu-no-such-binary", Vec::<&str>::new());
        let err = ProcessRunner.run(&spec).expect_err("expected not found");
        assert!(matches!(err, RunError::NotFound(_)), "got {err:?}");
    }

    #[test]
    fn process_runner_respects_dir_and_env() {
        let spec = CommandSpec::new("sh", ["-c", "pwd; printf %s \"$MU_T\""])
            .dir("/")
            .env("MU_T", "v");
        let output = ProcessRunner.run(&spec).expect("command should succeed");
        assert_eq!(output.stdout, b"/\nv");
        assert_eq!(output.exit_code, 0);
    }

    #[test]
    fn process_runner_look_path() {
        let path = ProcessRunner.look_path("sh").expect("sh must exist");
        assert!(is_executable(&path));
        let err = ProcessRunner
            .look_path("mu-no-such-binary")
            .expect_err("expected not found");
        assert!(matches!(err, RunError::NotFound(_)));
        // A file containing a separator is tried directly, no PATH search.
        let err = ProcessRunner
            .look_path("./mu-no-such-binary")
            .expect_err("expected not found");
        assert!(matches!(err, RunError::NotFound(_)));
    }

    // Go stub pattern: assert the exact argv the runner was asked for
    // (e.g. safety_test.go's cleanRunnerFunc checks name == "snap" &&
    // args == ["list", "--all"]).
    #[test]
    fn fake_runner_records_invocations() {
        let runner = FakeRunner::new();
        runner.push_output("ok\n", "", 0);

        let spec = CommandSpec::new("snap", ["list", "--all"]);
        let output = runner.run(&spec).expect("canned success");
        assert_eq!(output.stdout, b"ok\n");
        assert_eq!(runner.invocations(), vec![spec]);
    }

    // Go stub pattern: canned errors, e.g. errors.New("snapd unavailable").
    #[test]
    fn fake_runner_simulates_failures() {
        let runner = FakeRunner::new();
        runner.push_response(Err(RunError::Failed(Output {
            stdout: Vec::new(),
            stderr: b"snapd unavailable".to_vec(),
            exit_code: 1,
            signal: None,
        })));

        let spec = CommandSpec::new("snap", ["list"]);
        let err = runner.run(&spec).expect_err("expected canned failure");
        assert_eq!(err.output().unwrap().stderr, b"snapd unavailable");
        // The queue is drained: the next call gets the default Ok.
        runner.run(&spec).expect("default response");
        assert_eq!(runner.invocations().len(), 2);
    }

    // Go stub pattern: a closure dispatching on the command name.
    #[test]
    fn fake_runner_handler_dispatches_on_program() {
        let runner = FakeRunner::new();
        runner.set_handler(|spec| {
            if spec.program == "snap" {
                Ok(Output {
                    stdout: b"Name  Rev\n".to_vec(),
                    ..Output::default()
                })
            } else {
                Err(RunError::NotFound(
                    spec.program.to_string_lossy().into_owned(),
                ))
            }
        });

        assert_eq!(
            runner
                .run(&CommandSpec::new("snap", ["list"]))
                .unwrap()
                .stdout_lossy(),
            "Name  Rev\n"
        );
        let err = runner
            .run(&CommandSpec::new("flatpak", ["list"]))
            .expect_err("flatpak not stubbed");
        assert!(matches!(err, RunError::NotFound(_)));
    }

    #[test]
    fn fake_runner_look_path() {
        let runner = FakeRunner::new();
        runner.set_look_path("snap", "/usr/bin/snap");
        assert_eq!(
            runner.look_path("snap").unwrap(),
            PathBuf::from("/usr/bin/snap")
        );
        assert!(matches!(
            runner.look_path("gio"),
            Err(RunError::NotFound(_))
        ));
    }
}
