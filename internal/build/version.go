package build

// Version is the build/schema version embedded in both the client and daemon.
// Release builds override this value with Go ldflags.
var Version = "0055"

// Revision is the git commit SHA the running binary was built from. `make
// build`/`make build-install` set it automatically via ldflags to
// `git rev-parse HEAD`; it stays "unknown" for builds that skip the
// Makefile (plain `go build`, `go install`) or that ship a release tarball
// without embedding it. `afctl doctor` uses it to detect a daemon still
// running an older commit than the source checkout it was rebuilt from.
var Revision = "unknown"
