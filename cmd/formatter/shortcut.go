package formatter

import (
	"context"
	"fmt"
	"os"
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
type KEYBOARD_LOG_LEVEL int

const (
	NONE  KEYBOARD_LOG_LEVEL = 0
	INFO  KEYBOARD_LOG_LEVEL = 1
	DEBUG KEYBOARD_LOG_LEVEL = 2
)

type LogKeyboard struct {
	ErrorHandle           KeyboardError
	Watch                 KeyboardWatch
	started               bool
	IsDockerDesktopActive bool
	IsWatchConfigured     bool
	logLevel              KEYBOARD_LOG_LEVEL
	SignalChannel         chan<- os.Signal
	// services              []string
	// printerStop  func()
	// printerStart func()
}

var KeyboardManager *LogKeyboard
var eg multierror.Group
var errorColor = "\x1b[1;33m"

func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured bool, sc chan<- os.Signal, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error) {
	// func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured, startWatch bool, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error, stop func(), start func(), sc chan<- os.Signal) {
	km := LogKeyboard{}
	km.IsDockerDesktopActive = isDockerDesktopActive
	km.IsWatchConfigured = isWatchConfigured
	// km.printerStart = start
	// km.printerStop = stop
	km.logLevel = INFO
	// if up --watch and there is a watch config, we should start with watch running
	km.Watch.Watching = false
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
	switch lk.logLevel {
	case INFO:
		lk.createBuffer(2)
		lk.printInfo()
	case DEBUG:
		lk.createBuffer(3)
		// lk.printDebugOptions()
	}
}

func (lk *LogKeyboard) Error(prefix string, err error) {
	lk.ErrorHandle.errStart = time.Now()
	lk.ErrorHandle.err = fmt.Errorf("[%s]  %s", prefix, err.Error())
}

// This avoids incorrect printing at the end of the terminal
func (lk *LogKeyboard) createBuffer(lines int) {
	for i := 0; i < lines; i++ {
		fmt.Print("\033[K") // clear
		fmt.Print("\012")   // new line
	}
	fmt.Printf("\033[%dA", lines) // go back x lines
}

func (lk *LogKeyboard) printError(height int) {
	if lk.ErrorHandle.err != nil && int(time.Since(lk.ErrorHandle.errStart).Seconds()) < DISPLAY_ERROR_TIME {
		fmt.Printf("\033[%d;0H", height-1) // Move to before last line
		fmt.Printf("\033[K" + errorColor + lk.ErrorHandle.err.Error())
	}
}

func (lk *LogKeyboard) printInfo() {
	if lk.logLevel == INFO {
		height := goterm.Height()
		fmt.Print("\0337") // save cursor position
		lk.printError(height)
		fmt.Printf("\033[%d;0H", height) // Move to last line
		// clear line
		lk.infoMessage()
		fmt.Print("\0338") // restore cursor position
	}
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
	if lk.logLevel == INFO {
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
	if lk.logLevel == DEBUG {

	}
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
	fmt.Println("lk.isWatching()", lk.isWatching())
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

// func (lk *LogKeyboard) printDebugOptions() {
// 	if len(lk.services) == 0 {
// 		lk.logLevel = INFO
// 		return
// 	}
// 	height := goterm.Height()
// 	//clear
// 	fmt.Print("\0337") // save cursor position
// 	fmt.Printf("\033[%d;0H\033[2K", height-2)
// 	fmt.Printf("\033[%d;0H\033[2K", height-1)
// 	fmt.Printf("\033[%d;0H\033[2K", height)
// 	//clear

// 	fmt.Printf("\033[%d;0H", height-2) // Move to last line
// 	fmt.Print("\033[2K[Debug]  Select Service:")
// 	fmt.Printf("\033[%d;0H", height-1) // Move to last line
// 	serviceOpts := ""
// 	for i := 0; i < len(lk.services); i++ {
// 		serviceOpts = serviceOpts + fmt.Sprintf("(%d) %s   ", i+1, lk.services[i])
// 	}
// 	fmt.Printf("\033[2K%s", serviceOpts)
// 	fmt.Print("\0338") // restore cursor position
// }

// func (lk *LogKeyboard) debug(project *types.Project) {
// lk.services = project.ServiceNames()
// // show list
// lk.clearInfo()
// lk.logLevel = DEBUG
// // select
// // clear line
// choice := make(chan []byte)
// eg.Go(func() error {
// 	for {
// 		data := make([]byte, 8)
// 		n, err := os.Stdin.Read(data)

// 		// error handling : the basic thing to do is "on error, return"
// 		if err != nil {
// 			// if os.Stdin got closed, .Read() will return 'io.EOF'
// 			if err == io.EOF {
// 				log.Printf("stdin closed, exiting")
// 			} else {
// 				log.Printf("stdin: %s", err)
// 			}
// 			return err
// 		}
// 		// a habit to have : truncate your read buffers to 'n' after a .Read()
// 		choice <- data[:n]
// 		// return nil
// 	}
// })
// out := <-choice
// fmt.Println(out)
//run
// cmd := exec.Command("docker", "debug", "keep-not-latest-app-1")
// cmd.Stdout = os.Stdout
// cmd.Stdin = os.Stdin
// cmd.Stderr = os.Stderr

// lk.printerStop()
// if err := cmd.Start(); err != nil {
// 	lk.printerStart()
// 	lk.Error("Debug", fmt.Errorf("Could not start debug process."))
// 	return
// }
// err := cmd.Wait()
// lk.printerStart()
// lk.logLevel = INFO
// if err != nil {
// 	lk.Error("Debug", err)
// }
// }

func (lk *LogKeyboard) HandleKeyEvents(event keyboard.KeyEvent, ctx context.Context, project *types.Project, options api.UpOptions) {
	switch kRune := event.Rune; kRune {
	case 'V':
		lk.openDockerDesktop(project)
	case 'W':
		lk.StartWatch(ctx, project, options)
		// case 'D':
		// 	lk.debug(project)
	}
	switch key := event.Key; key {
	case keyboard.KeyCtrlC:
		keyboard.Close()
		lk.clearInfo()
		lk.logLevel = NONE
		if lk.Watch.Watching && lk.Watch.Cancel != nil {
			lk.Watch.Cancel()
			_ = eg.Wait().ErrorOrNil() // Need to print this ?
		}
		// will notify main thread to kill and will handle gracefully
		lk.SignalChannel <- syscall.SIGINT
	case keyboard.KeyEnter:
		lk.PrintEnter()
	}
}
