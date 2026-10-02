# Design

Auto-started dibsd owns a rotating structured-log writer from its first log
message. Store logs in `$XDG_STATE_HOME/dibs/logs` (fallback
`~/.local/state/dibs/logs`), with a SHA-256 of the cleaned socket path in the
filename to isolate custom instances. Keep at most 1 MiB in the active file
and one 1 MiB previous file. Truncate an oversized individual write to its tail.
Directory/file modes are 0700/0600; a per-log flock prevents concurrent starter
writers from racing rotation. Foreground/systemd/launchd retain stderr.

The CLI launches with null stdout/stderr instead of a socket-adjacent file;
normal startup failures are logged by dibsd to the bounded file. Error hints
read only the final 4 KiB of that file. Preflight the destination before spawn.
No pipe drainer tied to the short-lived CLI is needed. Unstructured child
stdout/stderr are discarded; structured daemon diagnostics are retained.

After a verified stop, acquire the existing database ownership lock before
removing the matching pid file and legacy `.startup.log`. Never clean artifacts
on an absent/unverified socket, nor delete retained state logs. No DB mutation,
new dependencies or live restart is involved.
