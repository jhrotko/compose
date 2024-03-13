package formatter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/buger/goterm"
	"github.com/docker/compose/v2/pkg/watch"
)

var DISPLAY_ERROR_TIME = 10

type LogKeyboard struct {
	err                   error
	errStart              time.Time
	started               bool
	IsDockerDesktopActive bool
	Watcher               watch.Notify
	Watching              bool
	Ctx                   context.Context
	Cancel                context.CancelFunc
}

var KeyboardManager = LogKeyboard{Watching: true}
var errorColor = "\x1b[1;33m"

func (lk *LogKeyboard) NewContext(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	lk.Ctx = ctx
	lk.Cancel = cancel
	return cancel
}

func (lk *LogKeyboard) PrintKeyboardInfo(print func()) {
	fmt.Print("\033[?25l")        // hide cursor
	defer fmt.Printf("\033[?25h") // show cursor

	if lk.started {
		lk.ClearInfo()
	} else {
		lk.started = true
	}
	print()
	lk.createBuffer()
	lk.printInfo()
}

func (lk *LogKeyboard) SError(err string) {
	lk.errStart = time.Now()
	lk.err = fmt.Errorf(err)
}
func (lk *LogKeyboard) Error(err error) {
	lk.errStart = time.Now()
	lk.err = err
}

// This avoids incorrect printing at the end of the terminal
func (lk *LogKeyboard) createBuffer() {
	fmt.Print("\012") // new line
	fmt.Print("\012")
	fmt.Print("\033[2A") // go back 3 lines
}

func (lk *LogKeyboard) printError(height int) {
	if lk.err != nil && int(time.Since(lk.errStart).Seconds()) < DISPLAY_ERROR_TIME {
		fmt.Printf("\033[%d;0H", height-1) // Move to before last line
		fmt.Printf("\033[K" + errorColor + "[Error]   " + lk.err.Error())
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
	if lk.IsDockerDesktopActive {
		options = options + keyColor("^V") + navColor("iew containers in Docker Desktop")
	}
	if lk.Watching {
		if strings.Contains(options, "Docker Desktop") {
			options = options + navColor(", ")
		}
		options = options + navColor("Enable ") + keyColor("^W") + navColor("atch Mode")
	}

	fmt.Print("\033[K" + options)
}

func (lk *LogKeyboard) ClearInfo() {
	height := goterm.Height()
	fmt.Print("\0337") // save cursor position
	if lk.err != nil {
		fmt.Printf("\033[%d;0H", height-1)
		fmt.Print("\033[2K") // clear line
	}
	fmt.Printf("\033[%d;0H", height) // Move to last line
	fmt.Print("\033[2K")             // clear line
	fmt.Print("\0338")               // restore cursor position
}

func (lk *LogKeyboard) PrintEnter() {
	lk.ClearInfo()
	lk.printInfo()
}

// func HandleKeyEvents(ctx context.Context, event keyboard.KeyEvent, project types.Project, options api.UpOptions, handleTearDown func()) {
// 	switch key := event.Key; key {
// 	case keyboard.KeyCtrlC:
// 		keyboard.Close()
// 		KeyboardManager.ClearInfo()
// 		handleTearDown()
// 	case keyboard.KeyCtrlG:
// 		if KeyboardManager.IsDockerDesktopActive {
// 			link := fmt.Sprintf("docker-desktop://dashboard/apps/%s", project.Name)
// 			err := open.Run(link)
// 			if err != nil {
// 				KeyboardManager.SError("Could not open Docker Desktop")
// 			} else {
// 				KeyboardManager.Error(nil)
// 			}
// 		}
// 	case keyboard.KeyCtrlW:
// 		if KeyboardManager.Watching {
// 			KeyboardManager.Watching = !KeyboardManager.Watching
// 			fmt.Println("watching shortcut", KeyboardManager.Watching)

// 			if KeyboardManager.Watching {
// 				KeyboardManager.Cancel()
// 			} else {
// 				KeyboardManager.NewContext(ctx)
// 				quit := make(chan error)
// 				go func() {
// 					buildOpts := *options.Create.Build
// 					buildOpts.Quiet = true
// 					err := s.Watch(KeyboardManager.Ctx, project, options.Start.Services, api.WatchOptions{
// 						Build: &buildOpts,
// 						LogTo: options.Start.Attach,
// 					})
// 					quit <- err
// 				}()
// 				KeyboardManager.Error(<-quit)
// 			}
// 		}
// 	case keyboard.KeyEnter:
// 		KeyboardManager.PrintEnter()
// 	default:
// 		if key != 0 { // If some key is pressed
// 			fmt.Println("key pressed: ", key)
// 		}
// 	}
// }
