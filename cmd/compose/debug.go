package compose

import (
	"context"
	"fmt"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

type debugOptions struct {
	*composeOptions

	service    string
	command    []string
	index      int
	privileged bool
	user       string
	workingDir string
}

func debugCommand(p *ProjectOptions, dockerCli command.Cli, backend api.Service) *cobra.Command {
	opts := debugOptions{
		composeOptions: &composeOptions{
			ProjectOptions: p,
		},
	}
	runCmd := &cobra.Command{
		Use:   "debug [OPTIONS] SERVICE",
		Short: "Execute a command in a running container.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: Adapt(func(ctx context.Context, args []string) error {
			opts.service = args[0]
			opts.command = args[1:]
			return nil
		}),
		RunE: Adapt(func(ctx context.Context, args []string) error {
			return runDebug(ctx, dockerCli, backend, opts)
		}),
		ValidArgsFunction: completeServiceNames(dockerCli, p),
	}

	runCmd.Flags().IntVar(&opts.index, "index", 0, "index of the container if service has multiple replicas")
	runCmd.Flags().BoolVarP(&opts.privileged, "privileged", "", false, "Give extended privileges to the process.")
	runCmd.Flags().StringVarP(&opts.user, "user", "u", "", "Run the command as this user.")
	runCmd.Flags().StringVarP(&opts.workingDir, "workdir", "w", "", "Path to workdir directory for this command.")

	return runCmd
}

func runDebug(ctx context.Context, dockerCli command.Cli, backend api.Service, options debugOptions) error {
	cmd := exec.Command("dld", "attach", options.service)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Started command")
	if err := cmd.Wait(); err != nil {
		return err
	}
	fmt.Println("Finished command")

	return nil
}
