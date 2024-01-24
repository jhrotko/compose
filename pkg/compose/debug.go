package compose

import (
	"errors"
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/context"
	"os"
	"os/exec"
	"strings"
)

type DebugConfig struct {
	Command    string
	Host       string
	Privileged bool
	Root       bool
	Shell      string
}

func (s *composeService) Debug(ctx context.Context, project *types.Project, options api.DebugOptions) error {
	config, err := loadDebugConfig(project.Services[options.Service])
	config.apply(options)
	if err != nil {
		return err
	}
	if config == nil {
		fmt.Println("Using default values")
	}
	args := make([]string, 5)
	args = convertFieldsToArgs("host", config.Host, args)
	args = convertFieldsToArgs("shell", config.Shell, args)
	args = convertFieldsToArgs("privileged", config.Privileged, args)
	args = convertFieldsToArgs("root", config.Root, args)
	args = convertFieldsToArgs("command", config.Command, args)
	//MISSING COMMAND
	fmt.Println(fmt.Sprintf("args yo %v", args))

	cmd := exec.Command("dld", "attach", options.Service)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Started command")
	if err := cmd.Wait(); err != nil {
		fmt.Println("Finished WITH ERROR command")
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return fmt.Errorf(exitError.Error())
		}
		return err
	}
	fmt.Println("Finished command")
	return nil
}

// Need to extend DevelopConfig to have DebugConfig in compose-go
// Once that is done, return type must be 8types.DevelopConfig instead of *DebugConfig
func loadDebugConfig(service types.ServiceConfig) (*DebugConfig, error) {
	var config DebugConfig
	//if service.Develop != nil {
	inputDebugMap, ok := service.Extensions["x-debug"]

	if !ok {
		fmt.Println("not ok")
		return &config, nil
	}
	if inputDebugMap == nil {
		fmt.Println("yo")
		return nil, nil
	}
	fmt.Println("service.Extensions", service.Extensions)
	err := mapstructure.Decode(inputDebugMap, &config)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Decode %#v\n", config)
	//}
	return &config, nil
}

// Override configuration with options from command line
func (d *DebugConfig) apply(opts api.DebugOptions) {
	fmt.Printf("apply opts %#v\n", opts)
	fmt.Printf("config %#v\n", *d)
	//if d.Privileged != opts.Privileged {
	//	d.Privileged = true
	//}
	//if !d.Root && opts.Root {
	//	d.Root = true
	//}
	if opts.Shell != "" {
		d.Shell = opts.Shell
	}
	if opts.Host != "" {
		d.Host = opts.Host
	}
	if opts.Command != "" {
		d.Command = opts.Command
	}
	fmt.Printf("END APPLY config %#v\n", *d)
}

func convertFieldsToArgs(field string, value interface{}, args []string) []string {
	switch value.(type) {
	case bool:
		if value == true {
			return append(args, fmt.Sprintf("--%s", strings.ToLower(field)))
		} else {
			return args
		}
	case string:
		if value == "" {
			return args
		}
	}
	args = append(args, fmt.Sprintf("--%s", strings.ToLower(field)))
	args = append(args, fmt.Sprint(value))
	fmt.Println(fmt.Sprintf("HMMM args %v", args))

	return args
}
