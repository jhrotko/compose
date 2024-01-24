package compose

import (
	"context"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/spf13/cobra"
)

type debugOptions struct {
	*ProjectOptions

	index   int
	service string
	Command string
	host    string
	shell   string
	//privileged bool
	//root       bool
}

func debugCommand(p *ProjectOptions, dockerCli command.Cli, backend api.Service) *cobra.Command {
	opts := debugOptions{
		ProjectOptions: p,
	}
	cmd := &cobra.Command{
		Use:   "debug [OPTIONS] SERVICE",
		Short: "Execute a command in a running container.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: Adapt(func(ctx context.Context, args []string) error {
			opts.service = args[0]
			return nil
		}),
		RunE: Adapt(func(ctx context.Context, args []string) error {
			return runDebug(ctx, dockerCli, backend, opts)
		}),
		ValidArgsFunction: completeServiceNames(dockerCli, p),
	}

	cmd.Flags().IntVar(&opts.index, "index", 0, "index of the container if service has multiple replicas")
	cmd.Flags().StringVar(&opts.shell, "shell", "", "Select a shell. Supported: \"bash\", \"fish\", \"zsh\", \"auto\". (default auto)")
	cmd.Flags().StringVar(&opts.host, "host", "", "Daemon docker socket to connect to. E.g.: 'ssh://root@example.org', 'unix:///some/path/docker.sock'")
	cmd.Flags().StringVarP(&opts.Command, "command", "c", "", "Evaluate the specified commands instead, passing additional positional arguments through $argv.")
	//cmd.Flags().BoolVar(&opts.privileged, "privileged", false, "Give extended privileges to the process.")
	//cmd.Flags().BoolVar(&opts.root, "root", false, "Attach as root user (uid = 0).")
	return cmd
}

func runDebug(ctx context.Context, dockerCli command.Cli, backend api.Service, opts debugOptions) error {
	debugOpts := api.DebugOptions{
		Command: opts.Command,
		Service: opts.service,
		Host:    opts.host,
		Shell:   opts.shell,
		//Privileged: opts.privileged,
		//Root:       opts.root,
	}
	project, err := opts.ToProject(dockerCli, []string{opts.service})
	if err != nil {
		return err
	}
	err = backend.Debug(ctx, project, debugOpts)
	return err
}
