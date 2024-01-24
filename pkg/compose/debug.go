package compose

import (
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
	args := []string{"debug", options.ContainerID}

	args = convertFieldsToArgs("host", config.Host, args)
	args = convertFieldsToArgs("shell", config.Shell, args)
	args = convertFieldsToArgs("command", config.Command, args)

	fmt.Printf("args %v\n", args)
	cmd := exec.Command("docker", args[0:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	//fmt.Println("Started command")
	if err := cmd.Wait(); err != nil {
		//var exitError *exec.ExitError
		//if errors.As(err, &exitError) {
		//	//fmt.Println("Finished WITH ERROR command")
		//	return fmt.Errorf(exitError.Error())
		//}
		//fmt.Println("Finished WITH ERROR!!")
		return err
	}
	//fmt.Println("Finished command")
	return nil
}

// Need to extend DevelopConfig to have DebugConfig in compose-go
// Once that is done, return type must be 8types.DevelopConfig instead of *DebugConfig
func loadDebugConfig(service types.ServiceConfig) (*DebugConfig, error) {
	var config DebugConfig
	//if service.Develop != nil {
	inputDebugMap, ok := service.Extensions["x-debug"]

	if !ok {
		fmt.Println("There is no compose configuration for debug")
		return &config, nil
	}
	if inputDebugMap == nil {
		fmt.Println("yo")
		return &config, nil
	}
	//fmt.Println("service.Extensions", service.Extensions)
	err := mapstructure.Decode(inputDebugMap, &config)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Decode %#v\n", config)
	//}
	return &config, nil
}

// Override configuration with options from command line
func (d *DebugConfig) apply(opts api.DebugOptions) {
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
}

func convertFieldsToArgs(field string, value interface{}, args []string) []string {
	switch value.(type) {
	//case bool:
	//	if value == true {
	//		return append(args, fmt.Sprintf("--%s", strings.ToLower(field)))
	//	} else {
	//		return args
	//	}
	case string:
		if value == "" {
			//fmt.Println("should be empty", field)
			return args
		}
		args = append(args, fmt.Sprintf("--%s", strings.ToLower(field)))
		args = append(args, fmt.Sprintf("%s", value))
	}
	return args
}
