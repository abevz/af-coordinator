package core

import "time"

type Health struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	DBPath     string    `json:"db_path"`
	SocketPath string    `json:"socket_path"`
	Time       time.Time `json:"time"`
	Version    string    `json:"version"`
	// Revision is the git commit SHA the running daemon binary was built
	// from ("unknown" if not embedded at build time). See build.Revision.
	Revision string `json:"revision,omitempty"`
}
