package command

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/basecamp/once/internal/background"
)

type BackgroundRunCommand struct {
	root *RootCommand
	cmd  *cobra.Command
}

func NewBackgroundRunCommand(root *RootCommand) *BackgroundRunCommand {
	b := &BackgroundRunCommand{root: root}
	b.cmd = &cobra.Command{
		Use:    "run",
		Short:  "Run background tasks (automatic backups and updates)",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE:   b.run,
	}
	return b
}

func (b *BackgroundRunCommand) Command() *cobra.Command {
	return b.cmd
}

// Private

func (b *BackgroundRunCommand) run(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	namespace, _ := cmd.Root().PersistentFlags().GetString("namespace")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runner := background.NewRunner(namespace, logger)

	return runner.Run(ctx)
}
