package systemd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Install(ctx context.Context, serviceName, unitContent string) error {
	path, err := unitFilePath(serviceName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating systemd user directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	if err := DaemonReload(ctx); err != nil {
		return err
	}

	return run(ctx, "enable", "--now", serviceName)
}

func Remove(ctx context.Context, serviceName string) error {
	if err := run(ctx, "disable", "--now", serviceName); err != nil {
		return err
	}

	path, err := unitFilePath(serviceName)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	return DaemonReload(ctx)
}

func DaemonReload(ctx context.Context) error {
	return run(ctx, "daemon-reload")
}

// Helpers

func run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl --user %s: %w", args[0], err)
	}
	return nil
}

func unitFilePath(serviceName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	return filepath.Join(home, ".config", "systemd", "user", serviceName+".service"), nil
}
