package command

import "github.com/spf13/cobra"

type BackgroundCommand struct {
	root *RootCommand
	cmd  *cobra.Command
}

func NewBackgroundCommand(root *RootCommand) *BackgroundCommand {
	b := &BackgroundCommand{root: root}
	b.cmd = &cobra.Command{
		Use:   "background",
		Short: "Manage background tasks (automatic backups and updates)",
	}

	b.cmd.AddCommand(NewBackgroundInstallCommand(root).Command())
	b.cmd.AddCommand(NewBackgroundUninstallCommand(root).Command())
	b.cmd.AddCommand(NewBackgroundRunCommand(root).Command())

	return b
}

func (b *BackgroundCommand) Command() *cobra.Command {
	return b.cmd
}
