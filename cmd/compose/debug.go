package compose

import (
	"context"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/spf13/cobra"
)

type debugOptions struct {
	*ProjectOptions

	Command    []string
	service    string
	index      int
	privileged bool
	root       bool
	host       string
	shell      string
}

func debugCommand(p *ProjectOptions, dockerCli command.Cli, backend api.Service) *cobra.Command {
	opts := debugOptions{
		ProjectOptions: p,
	}
	runCmd := &cobra.Command{
		Use:   "debug [OPTIONS] SERVICE",
		Short: "Execute a command in a running container.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: Adapt(func(ctx context.Context, args []string) error {
			opts.service = args[0]
			if len(args) > 1 {
				opts.Command = args[1:]
			}
			return nil
		}),
		RunE: Adapt(func(ctx context.Context, args []string) error {
			return runDebug(ctx, dockerCli, backend, opts)
		}),
		ValidArgsFunction: completeServiceNames(dockerCli, p),
	}

	runCmd.Flags().IntVar(&opts.index, "index", 0, "index of the container if service has multiple replicas")
	runCmd.Flags().StringVarP(&opts.host, "host", "", "", "Daemon docker socket to connect to. E.g.: 'ssh://root@example.org', 'unix:///some/path/docker.sock'")
	runCmd.Flags().BoolVarP(&opts.privileged, "privileged", "", false, "Give extended privileges to the process.")
	runCmd.Flags().BoolVarP(&opts.root, "root", "", false, "Attach as root user (uid = 0).")
	runCmd.Flags().StringVarP(&opts.shell, "shell", "", "", "Select a shell. Supported: \"bash\", \"fish\", \"zsh\", \"auto\". (default auto)")

	return runCmd
}

func runDebug(ctx context.Context, dockerCli command.Cli, backend api.Service, opts debugOptions) error {
	debugOpts := api.DebugOptions{
		Command:    opts.Command,
		Service:    opts.service,
		Host:       opts.host,
		Privileged: opts.privileged,
	}
	service := []string{opts.service}
	project, err := opts.ToProject(dockerCli, service)
	if err != nil {
		return err
	}
	err = backend.Debug(ctx, project, debugOpts)
	return err
}
