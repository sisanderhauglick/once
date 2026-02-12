package systemd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func IsInstalled(serviceName string) bool {
	_, err := os.Stat(unitFilePath(serviceName))
	return err == nil
}

func Install(ctx context.Context, serviceName, unitContent string) error {
	path := unitFilePath(serviceName)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating systemd directory: %w", err)
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
	if !IsInstalled(serviceName) {
		return nil
	}

	if err := run(ctx, "disable", "--now", serviceName); err != nil {
		return err
	}

	path := unitFilePath(serviceName)

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
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", args[0], err)
	}
	return nil
}

func unitFilePath(serviceName string) string {
	return filepath.Join("/etc/systemd/system", serviceName+".service")
}
