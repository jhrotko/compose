package formatter

import (
	"context"
	"fmt"
	"time"

	"github.com/buger/goterm"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/watch"
	"github.com/eiannone/keyboard"
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
	ErrorHandle           KeyboardError
	Watch                 KeyboardWatch
	started               bool
	IsDockerDesktopActive bool
	IsWatchConfigured     bool
}

var KeyboardManager *LogKeyboard

var errorColor = "\x1b[1;33m"

func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured bool, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error) {
	km := LogKeyboard{}
	KeyboardManager = &km
	KeyboardManager.Watch.Watching = true
	KeyboardManager.IsDockerDesktopActive = isDockerDesktopActive
	KeyboardManager.IsWatchConfigured = isWatchConfigured
	KeyboardManager.Watch.WatchFn = watchFn
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
	options = options + openDDInfo + watchInfo

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
	if lk.IsDockerDesktopActive {
		link := fmt.Sprintf("docker-desktop://dashboard/apps/%s", project.Name)
		err := open.Run(link)
		if err != nil {
			lk.Error("View", fmt.Errorf("could not open Docker Desktop"))
		} else {
			lk.Error("", nil)
		}
	}
}

func (lk *LogKeyboard) StartWatch(ctx context.Context, project *types.Project, options api.UpOptions) {
	if !lk.IsWatchConfigured {
		lk.Error("Watch", fmt.Errorf("Watch is not yet configured. Learn more: https://docs.docker.com/compose/file-watch/"))
		return
	}
	lk.switchWatching()
	if lk.isWatching() {
		fmt.Println("watching shortcut")
		lk.Watch.Cancel()
	} else {
		lk.newContext(ctx)
		errW := make(chan error)
		go func() {
			buildOpts := *options.Create.Build
			buildOpts.Quiet = true
			err := lk.Watch.WatchFn(lk.Watch.Ctx, project, options.Start.Services, api.WatchOptions{
				Build: &buildOpts,
				LogTo: options.Start.Attach,
			})
			errW <- err
		}()
		lk.Error("Watch", <-errW)
	}
}

func (lk *LogKeyboard) HandleKeyEvents(ctx context.Context, event keyboard.KeyEvent, project *types.Project, options api.UpOptions, handleTearDown func()) {
	switch kRune := event.Rune; kRune {
	case 'V':
		lk.openDockerDesktop(project)
	case 'W':
		lk.StartWatch(ctx, project, options)
	}
	switch key := event.Key; key {
	case keyboard.KeyCtrlC:
		keyboard.Close()
		lk.clearInfo()
		handleTearDown()
	case keyboard.KeyEnter:
		lk.PrintEnter()
	default:
		if key != 0 { // If some key is pressed
			fmt.Println("key pressed: ", key)
		}
	}
}
