package command

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/systemd"
)

type BackgroundUninstallCommand struct {
	root *RootCommand
	cmd  *cobra.Command
}

func NewBackgroundUninstallCommand(root *RootCommand) *BackgroundUninstallCommand {
	b := &BackgroundUninstallCommand{root: root}
	b.cmd = &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the background tasks systemd user service",
		Args:  cobra.NoArgs,
		RunE:  b.run,
	}
	return b
}

func (b *BackgroundUninstallCommand) Command() *cobra.Command {
	return b.cmd
}

// Private

func (b *BackgroundUninstallCommand) run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	namespace, _ := cmd.Root().PersistentFlags().GetString("namespace")
	serviceName := namespace + "-background"

	if err := systemd.Remove(ctx, serviceName); err != nil {
		return fmt.Errorf("removing service: %w", err)
	}

	fmt.Printf("Uninstalled %s.service\n", serviceName)
	return nil
}
