package main

import (
	"fmt"
	"os"
	"time"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
)

func main() {
	cfg := config.Default()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "health":
		c := client.New(cfg.SocketPath)
		health, err := c.Health()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Name:       %s\n", health.Name)
		fmt.Printf("Status:     %s\n", health.Status)
		fmt.Printf("DBPath:     %s\n", health.DBPath)
		fmt.Printf("SocketPath: %s\n", health.SocketPath)
		fmt.Printf("Time:       %s\n", health.Time.UTC().Format(time.RFC3339))
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: afctl <command>\n\nCommands:\n  health  Check daemon health\n")
}
