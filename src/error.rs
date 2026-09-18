//! Shared error type for the crate — mirrors the Go code's `fmt.Errorf`
//! string errors plus the one structured error (`TrashRecoveryError`).

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error(transparent)]
    Io(#[from] std::io::Error),
    /// Errors produced with Go `fmt.Errorf` semantics — message text is
    /// user-visible and kept byte-compatible where it matters.
    #[error("{0}")]
    Msg(String),
    /// `TrashRecoveryError` — reports where data can be recovered when trash
    /// metadata finalization and rollback both fail.
    #[error(
        "trash metadata failed for {original}; rollback failed; recover data from {recovery}: metadata: {metadata}; rollback: {rollback}"
    )]
    TrashRecovery {
        original: String,
        recovery: String,
        metadata: String,
        rollback: String,
    },
    /// `fmt.Errorf("{context}: %w", source)` — wraps while keeping the inner
    /// variant reachable (the way `errors.As` still finds
    /// `*TrashRecoveryError` under Go's `fmt.Errorf("trash %s: %w", ...)`).
    #[error("{context}: {source}")]
    WithContext {
        context: String,
        #[source]
        source: Box<Error>,
    },
}

impl Error {
    /// Attach `context` the way Go callers do with `fmt.Errorf("...: %w", err)`;
    /// the wrapped variant stays inspectable via [`Error::source`].
    pub fn with_context(self, context: String) -> Error {
        Error::WithContext {
            context,
            source: Box::new(self),
        }
    }
}

pub type Result<T> = std::result::Result<T, Error>;

/// Build a message error, mirroring `fmt.Errorf` usage.
pub fn msg<T>(m: impl Into<String>) -> Result<T> {
    Err(Error::Msg(m.into()))
}
