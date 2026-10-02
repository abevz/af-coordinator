package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/abevz/dibs/internal/client"
)

// RestartDaemon changes no service configuration or coordinator storage. The
// caller invokes it only after its independent restart confirmation.
func RestartDaemon(ctx context.Context, socket, expected string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.CommandContext(ctx, "systemctl", "--user", "restart", "dibsd")
		cmd.Env = os.Environ()
		root := os.Getenv("XDG_RUNTIME_DIR")
		if root == "" {
			root = fmt.Sprintf("/run/user/%d", os.Getuid())
			cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR="+root)
		}
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			cmd.Env = append(cmd.Env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(root, "bus"))
		}
	case "darwin":
		cmd = exec.CommandContext(ctx, "launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/com.abevz.dibsd", os.Getuid()))
	default:
		return fmt.Errorf("unsupported daemon service platform")
	}
	if e := cmd.Run(); e != nil {
		return e
	}
	c := client.New(socket)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		h, e := c.Health(ctx)
		if e == nil && h.Version == expected {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("daemon did not report expected version %s: %w", expected, ctx.Err())
		case <-ticker.C:
		}
	}
}
