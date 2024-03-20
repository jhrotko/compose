/*
   Copyright 2020 Docker Compose CLI authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package compose

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli"
	"github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/internal/tracing"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/progress"
	"github.com/eiannone/keyboard"
	"github.com/hashicorp/go-multierror"
)

func (s *composeService) Up(ctx context.Context, project *types.Project, options api.UpOptions) error { //nolint:gocyclo
	err := progress.Run(ctx, tracing.SpanWrapFunc("project/up", tracing.ProjectOptions(ctx, project), func(ctx context.Context) error {
		w := progress.ContextWriter(ctx)
		w.HasMore(options.Start.Attach == nil)
		err := s.create(ctx, project, options.Create)
		if err != nil {
			return err
		}
		if options.Start.Attach == nil {
			w.HasMore(false)
			return s.start(ctx, project.Name, options.Start, nil)
		}
		return nil
	}), s.stdinfo())
	if err != nil {
		return err
	}

	if options.Start.Attach == nil {
		return err
	}
	if s.dryRun {
		fmt.Fprintln(s.stdout(), "end of 'compose up' output, interactive run is not supported in dry-run mode")
		return err
	}

	var eg multierror.Group

	// if we get a second signal during shutdown, we kill the services
	// immediately, so the channel needs to have sufficient capacity or
	// we might miss a signal while setting up the second channel read
	// (this is also why signal.Notify is used vs signal.NotifyContext)
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer close(signalChan)
	var isTerminated bool
	printer := newLogPrinter(options.Start.Attach)

	doneCh := make(chan bool)
	eg.Go(func() error {
		kEvents, err := keyboard.GetKeys(100)
		if err != nil {
			panic(err)
		}
		defer keyboard.Close()
		first := true
		gracefulTeardown := func() {
			printer.Cancel()
			fmt.Fprintln(s.stdinfo(), "\033[KGracefully stopping... (press Ctrl+C again to force)")
			eg.Go(func() error {
				err := s.Stop(context.Background(), project.Name, api.StopOptions{
					Services: options.Create.Services,
					Project:  project,
				})
				isTerminated = true
				close(doneCh)
				return err
			})
			first = false
		}
		for {
			select {
			case event := <-kEvents:
				switch key := event.Key; key {
				case keyboard.KeyCtrlC:
					keyboard.Close()
					formatter.KeyboardInfo.ClearInfo()
					gracefulTeardown()
				case keyboard.KeyCtrlG:
					link := fmt.Sprintf("docker-desktop://dashboard/apps/%s", project.Name)
					// err := fmt.Errorf("OH NO!\n")
					// if err != nil {
					// 	fmt.Print("\0337")                          // save cursor position
					// 	fmt.Println("\033[0;0H")                    // Move to top
					// 	fmt.Printf("\033[0;34m")                    //change color
					// 	fmt.Printf("\033[%d;0H", goterm.Height()-1) // Move to last line
					// 	fmt.Printf("\033[K%s", err.Error())
					// 	// fmt.Println("\033[0m") // restore color
					// 	fmt.Println("\033[u") //restore
					// }
					// err := open.Run(link)
					fmt.Println("link: ", link)
				// if err != nil {
				// 	fmt.Fprintln(s.stdinfo(), "Could not open Docker Desktop")
				// }
				case keyboard.KeyEnter:
					formatter.KeyboardInfo.PrintEnter()
				default:
					if key != 0 {
						fmt.Println("key pressed: ", key)
					}
				}
			case <-doneCh:
				return nil
			case <-ctx.Done():
				if first {
					gracefulTeardown()
				}
			case <-signalChan:
				if first {
					gracefulTeardown()
				} else {
					eg.Go(func() error {
						return s.Kill(context.Background(), project.Name, api.KillOptions{
							Services: options.Create.Services,
							Project:  project,
						})
					})
					return nil
				}
			}
		}
	})

	var exitCode int
	eg.Go(func() error {
		code, err := printer.Run(options.Start.CascadeStop, options.Start.ExitCodeFrom, func() error {
			fmt.Fprintln(s.stdinfo(), "Aborting on container exit...")
			return progress.Run(ctx, func(ctx context.Context) error {
				return s.Stop(ctx, project.Name, api.StopOptions{
					Services: options.Create.Services,
					Project:  project,
				})
			}, s.stdinfo())
		})
		exitCode = code
		return err
	})

	if options.Start.Watch {
		eg.Go(func() error {
			buildOpts := *options.Create.Build
			buildOpts.Quiet = true
			return s.Watch(ctx, project, options.Start.Services, api.WatchOptions{
				Build: &buildOpts,
				LogTo: options.Start.Attach,
			})
		})
	}

	// We don't use parent (cancelable) context as we manage sigterm to stop the stack
	err = s.start(context.Background(), project.Name, options.Start, printer.HandleEvent)
	if err != nil && !isTerminated { // Ignore error if the process is terminated
		return err
	}

	printer.Stop()

	if !isTerminated {
		// signal for the signal-handler goroutines to stop
		close(doneCh)
	}
	err = eg.Wait().ErrorOrNil()
	if exitCode != 0 {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		return cli.StatusError{StatusCode: exitCode, Status: errMsg}
	}
	return err
}
