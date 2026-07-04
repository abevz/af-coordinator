package core

import "time"

type Health struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	DBPath     string    `json:"db_path"`
	SocketPath string    `json:"socket_path"`
	Time       time.Time `json:"time"`
	Version    string    `json:"version"`
}
