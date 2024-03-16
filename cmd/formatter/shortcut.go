package formatter

import (
	"context"
	"fmt"
	"math"
	"os"
	"syscall"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/buger/goterm"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/internal/tracing"
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
	IsDockerDesktopActive bool
	IsWatchConfigured     bool
	logLevel              KEYBOARD_LOG_LEVEL
	SignalChannel         chan<- os.Signal
	// services              []string
	// printerStop  func()
	// printerStart func()
	metrics tracing.KeyboardMetrics
}

var KeyboardManager *LogKeyboard
var eg multierror.Group
var errorColor = "\x1b[1;33m"

func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured bool, sc chan<- os.Signal, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error) {
	km := LogKeyboard{}
	km.IsDockerDesktopActive = isDockerDesktopActive
	km.IsWatchConfigured = isWatchConfigured
	// km.printerStart = start
	// km.printerStop = stop
	km.logLevel = INFO
	km.Watch.Watching = false
	km.Watch.WatchFn = watchFn
	km.SignalChannel = sc
	km.metrics = tracing.KeyboardMetrics{
		EnabledViewDockerDesktop: isDockerDesktopActive,
		HasWatchConfig:           isWatchConfigured,
	}
	KeyboardManager = &km
}

func (lk *LogKeyboard) PrintKeyboardInfo(print func()) {
	fmt.Print("\033[?25l")        // hide cursor
	defer fmt.Printf("\033[?25h") // show cursor

	lk.clearInfo()
	print()

	switch lk.logLevel {
	case INFO:
		lk.createBuffer(0)
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

func allocateSpace(lines int) {
	for i := 0; i < lines; i++ {
		fmt.Print("\033[2K")
		fmt.Print("\012") // new line
		fmt.Print("\033[0G")
	}
}

// This avoids incorrect printing at the end of the terminal
func (lk *LogKeyboard) createBuffer(lines int) {
	allocateSpace(lines)
	if lk.ErrorHandle.shoudlDisplay() && isOverflow(lk.ErrorHandle.err.Error()) {
		extraLines := linesOffset(lk.ErrorHandle.err.Error()) + 1
		allocateSpace(extraLines)
		lines = lines + extraLines
	}
	infoMessage := lk.infoMessage()
	if isOverflow(infoMessage) {
		extraLines := linesOffset(infoMessage) + 1
		allocateSpace(extraLines)
		lines = lines + extraLines
	}
	if lines > 0 {
		fmt.Printf("\033[%dA", lines) // go back x lines
	}
}

func (ke *KeyboardError) shoudlDisplay() bool {
	return ke.err != nil && int(time.Since(ke.errStart).Seconds()) < DISPLAY_ERROR_TIME
}

func isOverflow(s string) bool {
	return len(stripansi.Strip(s)) > goterm.Width()
}

func linesOffset(s string) int {
	return int(math.Floor(float64(len(stripansi.Strip(s))) / float64(goterm.Width())))
}

func (lk *LogKeyboard) printError(height int, info string) {
	if lk.ErrorHandle.shoudlDisplay() {
		errMessage := lk.ErrorHandle.err.Error()
		fmt.Printf("\033[%d;0H\033[2K", height-linesOffset(info)-linesOffset(errMessage)-1) // Move to before last line
		fmt.Printf(errorColor + errMessage + "\033[0m")
	}
}

func (lk *LogKeyboard) printInfo() {
	if lk.logLevel == INFO {
		height := goterm.Height()
		fmt.Print("\033[0G") // reset column position
		fmt.Print("\0337")   // save cursor position

		info := lk.infoMessage()
		lk.printError(height, info)

		fmt.Printf("\033[%d;0H\033[2K", height-linesOffset(info)) // Move to last line
		fmt.Print(info + "\033[0m")

		fmt.Print("\033[0G") // reset column position
		fmt.Print("\0338")
	}
}

func shortcutKeyColor(key string) string {
	foreground := "38;2;"
	black := "0;0;0"
	background := "48;2;"
	white := "255;255;255"
	bold := ";1"
	reset := "\033[0m"
	return "\033[" + foreground + black + ";" + background + white + bold + "m" + key + reset
}

func (lk *LogKeyboard) infoMessage() string {
	var options string
	var openDDInfo string
	if lk.IsDockerDesktopActive {
		openDDInfo = shortcutKeyColor("V") + navColor(" View in Docker Desktop")
	}
	var watchInfo string
	if openDDInfo != "" {
		watchInfo = navColor("   ")
	}
	var isEnabled = " Enable"
	if lk.Watch.Watching {
		isEnabled = " Disable"
	}
	watchInfo = watchInfo + shortcutKeyColor("V") + navColor(isEnabled+" Watch")
	return options + openDDInfo + watchInfo
}

func (lk *LogKeyboard) clearInfo() {
	height := goterm.Height()
	fmt.Print("\033[0G")
	fmt.Print("\0337") // clear from current cursor position
	for i := 0; i < height; i++ {
		fmt.Print("\033[1B\033[2K") //does not add new lines, so its ok
	}
	fmt.Print("\0338")
	// fmt.Println("\033[0J")
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
	lk.metrics.ActivateViewDockerDesktop = true
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
		lk.metrics.ActivateWatch = true
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
		go func() {
			tracing.SpanWrapFunc("navigation_menu", tracing.KeyboardOptions(lk.metrics),
				func(ctx context.Context) error {
					return nil
				})(ctx)
		}()

		// will notify main thread to kill and will handle gracefully
		lk.SignalChannel <- syscall.SIGINT
	case keyboard.KeyEnter:
		lk.PrintEnter()
	case keyboard.KeyCtrlL:
		height := goterm.Height()
		for i := 0; i < height; i++ {
			fmt.Println()
			fmt.Print("\033[2K")
		}
	}
}
