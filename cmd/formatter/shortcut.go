package formatter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/buger/goterm"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/watch"
	"github.com/eiannone/keyboard"
	"github.com/hashicorp/go-multierror"
	"github.com/skratchdot/open-golang/open"
)

var DISPLAY_ERROR_TIME = 10

type KeyboardError struct {
	err      error
	errStart time.Time
}
type KeyboardWatch struct {
	Watcher  watch.Notify
	Watching bool
	WatchFn  func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error
	Ctx      context.Context
	Cancel   context.CancelFunc
}
type LogKeyboard struct {
	SignalChannel         chan<- os.Signal
	ErrorHandle           KeyboardError
	Watch                 KeyboardWatch
	started               bool
	IsDockerDesktopActive bool
	IsWatchConfigured     bool
	printerStop           func()
	printerStart          func()
}

var KeyboardManager *LogKeyboard
var eg multierror.Group
var errorColor = "\x1b[1;33m"

func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured, startWatch bool, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error, stop func(), start func(), sc chan<- os.Signal) {
	km := LogKeyboard{}
	km.IsDockerDesktopActive = isDockerDesktopActive
	km.IsWatchConfigured = isWatchConfigured
	km.printerStart = start
	km.printerStop = stop
	// if up --watch and there is a watch config, we should start with watch running
	km.Watch.Watching = isWatchConfigured && startWatch
	km.Watch.WatchFn = watchFn
	km.SignalChannel = sc
	KeyboardManager = &km
}

func (lk *LogKeyboard) PrintKeyboardInfo(print func()) {
	fmt.Print("\033[?25l")        // hide cursor
	defer fmt.Printf("\033[?25h") // show cursor

	if lk.started {
		lk.clearInfo()
	} else {
		lk.started = true
	}
	print()
	lk.createBuffer()
	lk.printInfo()
}

func (lk *LogKeyboard) Error(prefix string, err error) {
	lk.ErrorHandle.errStart = time.Now()
	lk.ErrorHandle.err = fmt.Errorf("[%s]  %s", prefix, err.Error())
}

// This avoids incorrect printing at the end of the terminal
func (lk *LogKeyboard) createBuffer() {
	fmt.Print("\012") // new line
	fmt.Print("\012")
	fmt.Print("\033[2A") // go back 3 lines
}

func (lk *LogKeyboard) printError(height int) {
	if lk.ErrorHandle.err != nil && int(time.Since(lk.ErrorHandle.errStart).Seconds()) < DISPLAY_ERROR_TIME {
		fmt.Printf("\033[%d;0H", height-1) // Move to before last line
		fmt.Printf("\033[K" + errorColor + lk.ErrorHandle.err.Error())
	}
}

func (lk *LogKeyboard) printInfo() {
	height := goterm.Height()
	fmt.Print("\0337") // save cursor position
	lk.printError(height)
	fmt.Printf("\033[%d;0H", height) // Move to last line
	// clear line
	lk.infoMessage()
	fmt.Print("\0338") // restore cursor position
}

func (lk *LogKeyboard) infoMessage() {
	options := navColor("  Options:  ")
	var openDDInfo string
	if lk.IsDockerDesktopActive {
		openDDInfo = keyColor("^V") + navColor("iew containers in Docker Desktop")
	}
	var watchInfo string
	if openDDInfo != "" {
		watchInfo = navColor(", ")
	}
	watchInfo = watchInfo + navColor("Enable ") + keyColor("^W") + navColor("atch Mode")
	debugOptions := navColor(", ") + keyColor("^D") + navColor("ebug")
	options = options + openDDInfo + watchInfo + debugOptions

	fmt.Print("\033[K" + options)
}

func (lk *LogKeyboard) clearInfo() {
	height := goterm.Height()
	fmt.Print("\0337") // save cursor position
	if lk.ErrorHandle.err != nil {
		fmt.Printf("\033[%d;0H", height-1)
		fmt.Print("\033[2K") // clear line
	}
	fmt.Printf("\033[%d;0H", height) // Move to last line
	fmt.Print("\033[2K")             // clear line
	fmt.Print("\0338")               // restore cursor position
}

func (lk *LogKeyboard) PrintEnter() {
	lk.clearInfo()
	lk.printInfo()
}

func (lk *LogKeyboard) isWatching() bool {
	return lk.Watch.Watching
}

func (lk *LogKeyboard) switchWatching() {
	lk.Watch.Watching = !lk.Watch.Watching
}

func (lk *LogKeyboard) newContext(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	lk.Watch.Ctx = ctx
	lk.Watch.Cancel = cancel
	return cancel
}

func (lk *LogKeyboard) openDockerDesktop(project *types.Project) {
	if !lk.IsDockerDesktopActive {
		return
	}
	link := fmt.Sprintf("docker-desktop://dashboard/apps/%s", project.Name)
	err := open.Run(link)
	if err != nil {
		lk.Error("View", fmt.Errorf("Could not open Docker Desktop"))

	}
}

func (lk *LogKeyboard) StartWatch(ctx context.Context, project *types.Project, options api.UpOptions) {
	if !lk.IsWatchConfigured {
		lk.Error("Watch", fmt.Errorf("Watch is not yet configured. Learn more: https://docs.docker.com/compose/file-watch/"))
		return
	}
	lk.switchWatching()
	if !lk.isWatching() && lk.Watch.Cancel != nil {
		lk.Watch.Cancel()
	} else {
		lk.newContext(ctx)
		eg.Go(func() error {
			buildOpts := *options.Create.Build
			buildOpts.Quiet = true
			err := lk.Watch.WatchFn(lk.Watch.Ctx, project, options.Start.Services, api.WatchOptions{
				Build: &buildOpts,
				LogTo: options.Start.Attach,
			})
			// fmt.Println("sent error")
			return err
		})
	}
}

func (lk *LogKeyboard) debug() {
	cmd := exec.Command("docker", "debug", "keep-not-latest-app-1")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	lk.printerStop()
	if err := cmd.Start(); err != nil {
		lk.printerStart()
		lk.Error("Debug", fmt.Errorf("Could not start debug process."))
		return
	}
	err := cmd.Wait()
	lk.printerStart()
	if err != nil {
		lk.Error("Debug", err)
	}
}

func (lk *LogKeyboard) HandleKeyEvents(event keyboard.KeyEvent, ctx context.Context, project *types.Project, options api.UpOptions) {
	switch kRune := event.Rune; kRune {
	case 'V':
		lk.openDockerDesktop(project)
	case 'W':
		lk.StartWatch(ctx, project, options)
	case 'D':
		lk.debug()
	}
	switch key := event.Key; key {
	case keyboard.KeyCtrlC:
		keyboard.Close()
		lk.clearInfo()
		if lk.Watch.Watching && lk.Watch.Cancel != nil {
			// fmt.Println("canceling")
			lk.Watch.Cancel()
			// fmt.Println("canceling watch?")
			err := eg.Wait().ErrorOrNil() // Need to print this ?
			fmt.Println("done@", err)
		}
		// will notify main thread to kill and will handle gracefully
		// fmt.Println("tear down")
		signal.Notify(lk.SignalChannel, syscall.SIGTERM)
	case keyboard.KeyEnter:
		lk.PrintEnter()
	}
}
