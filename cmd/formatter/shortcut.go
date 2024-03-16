/*
   Copyright 2024 Docker Compose CLI authors

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

package formatter

import (
	"context"
	"fmt"
	"math"
	"os"
	"syscall"
	"time"

	"github.com/buger/goterm"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/internal/tracing"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/watch"
	"github.com/eiannone/keyboard"
	"github.com/hashicorp/go-multierror"
	"github.com/skratchdot/open-golang/open"
)

const DISPLAY_ERROR_TIME = 10

type KeyboardError struct {
	err       error
	timeStart time.Time
}

func (ke *KeyboardError) shoudlDisplay() bool {
	return ke.err != nil && int(time.Since(ke.timeStart).Seconds()) < DISPLAY_ERROR_TIME
}

func (ke *KeyboardError) printError(height int, info string) {
	if ke.shoudlDisplay() {
		errMessage := ke.err.Error()

		MoveCursor(height-linesOffset(info)-linesOffset(errMessage)-1, 0)
		ClearLine()

		fmt.Print(errMessage)
	}
}

func (ke *KeyboardError) addError(prefix string, err error) {
	ke.timeStart = time.Now()

	prefix = ansiColor("1;36", fmt.Sprintf("%s →", prefix))
	errorString := fmt.Sprintf("%s  %s", prefix, err.Error())

	ke.err = fmt.Errorf(errorString)
}

func (ke *KeyboardError) error() string {
	return ke.err.Error()
}

type KeyboardWatch struct {
	Watcher  watch.Notify
	Watching bool
	WatchFn  func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error
	Ctx      context.Context
	Cancel   context.CancelFunc
}

func (kw *KeyboardWatch) isWatching() bool {
	return kw.Watching
}

func (kw *KeyboardWatch) switchWatching() {
	kw.Watching = !kw.Watching
}

func (kw *KeyboardWatch) newContext(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	kw.Ctx = ctx
	kw.Cancel = cancel
	return cancel
}

type KEYBOARD_LOG_LEVEL int

const (
	NONE  KEYBOARD_LOG_LEVEL = 0
	INFO  KEYBOARD_LOG_LEVEL = 1
	DEBUG KEYBOARD_LOG_LEVEL = 2
)

type LogKeyboard struct {
	kError                KeyboardError
	Watch                 KeyboardWatch
	IsDockerDesktopActive bool
	IsWatchConfigured     bool
	logLevel              KEYBOARD_LOG_LEVEL
	signalChannel         chan<- os.Signal
	metrics               tracing.KeyboardMetrics
}

var KeyboardManager *LogKeyboard
var eg multierror.Group

func NewKeyboardManager(isDockerDesktopActive, isWatchConfigured bool, sc chan<- os.Signal, watchFn func(ctx context.Context, project *types.Project, services []string, options api.WatchOptions) error) {
	km := LogKeyboard{}
	km.IsDockerDesktopActive = isDockerDesktopActive
	km.IsWatchConfigured = isWatchConfigured
	km.logLevel = INFO

	km.Watch.Watching = false
	km.Watch.WatchFn = watchFn

	km.signalChannel = sc

	km.metrics = tracing.KeyboardMetrics{
		EnabledViewDockerDesktop: isDockerDesktopActive,
		HasWatchConfig:           isWatchConfigured,
	}

	KeyboardManager = &km

	HideCursor()
}

func (lk *LogKeyboard) PrintKeyboardInfo(print func()) {
	lk.clearNavigationMenu()
	print()

	if lk.logLevel == INFO {
		lk.createBuffer(1)
		lk.printNavigationMenu()
	}
}

// Creates space to print error and menu string
func (lk *LogKeyboard) createBuffer(lines int) {
	allocateSpace(lines)

	if lk.kError.shoudlDisplay() && isOverflow(lk.kError.error()) {
		extraLines := linesOffset(lk.kError.error()) + 1
		allocateSpace(extraLines)
		lines = lines + extraLines
	}

	infoMessage := lk.navigationMenu()
	if isOverflow(infoMessage) {
		extraLines := linesOffset(infoMessage) + 1
		allocateSpace(extraLines)
		lines = lines + extraLines
	}

	if lines > 0 {
		MoveCursorUp(lines)
	}
}

func (lk *LogKeyboard) printNavigationMenu() {
	if lk.logLevel == INFO {
		height := goterm.Height()
		menu := lk.navigationMenu()

		MoveCursorX(0)
		SaveCursor()

		lk.kError.printError(height, menu)

		MoveCursor(height-linesOffset(menu), 0)
		ClearLine()
		fmt.Print(menu)

		MoveCursorX(0)
		RestoreCursor()
	}
}

func (lk *LogKeyboard) navigationMenu() string {
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
	watchInfo = watchInfo + shortcutKeyColor("W") + navColor(isEnabled+" Watch")
	return options + openDDInfo + watchInfo
}

func (lk *LogKeyboard) clearNavigationMenu() {
	height := goterm.Height()
	MoveCursorX(0)
	SaveCursor()
	for i := 0; i < height; i++ {
		MoveCursorDown(1)
		ClearLine()
	}
	RestoreCursor()
}

func (lk *LogKeyboard) PrintEnter() {
	lk.clearNavigationMenu()
	lk.printNavigationMenu()
}

func (lk *LogKeyboard) CleanTerminal() {
	height := goterm.Height()
	for i := 0; i < height; i++ {
		NewLine()
		ClearLine()
	}
}

func (lk *LogKeyboard) openDockerDesktop(project *types.Project) {
	if !lk.IsDockerDesktopActive {
		return
	}
	lk.metrics.ActivateViewDockerDesktop = true
	link := fmt.Sprintf("docker-desktop://dashboard/apps/%s", project.Name)
	err := open.Run(link)
	if err != nil {
		lk.kError.addError("View", fmt.Errorf("Could not open Docker Desktop"))
	}
}

func (lk *LogKeyboard) StartWatch(ctx context.Context, project *types.Project, options api.UpOptions) {
	if !lk.IsWatchConfigured {
		lk.kError.addError("Watch", fmt.Errorf("Watch is not yet configured. Learn more: %s", ansiColor("36", "https://docs.docker.com/compose/file-watch/")))
		return
	}
	lk.Watch.switchWatching()
	if !lk.Watch.isWatching() && lk.Watch.Cancel != nil {
		lk.Watch.Cancel()
	} else {
		lk.Watch.newContext(ctx)
		eg.Go(func() error {
			buildOpts := *options.Create.Build
			buildOpts.Quiet = true
			err := lk.Watch.WatchFn(lk.Watch.Ctx, project, options.Start.Services, api.WatchOptions{
				Build: &buildOpts,
				LogTo: options.Start.Attach,
			})
			return err
		})
	}
}

func (lk *LogKeyboard) HandleKeyEvents(event keyboard.KeyEvent, ctx context.Context, project *types.Project, options api.UpOptions) {
	switch kRune := event.Rune; kRune {
	case 'V':
		lk.openDockerDesktop(project)
	case 'W':
		lk.metrics.ActivateWatch = true
		lk.StartWatch(ctx, project, options)
	}
	switch key := event.Key; key {
	case keyboard.KeyCtrlC:
		keyboard.Close()
		lk.clearNavigationMenu()
		lk.logLevel = NONE
		if lk.Watch.Watching && lk.Watch.Cancel != nil {
			lk.Watch.Cancel()
			_ = eg.Wait().ErrorOrNil() // Need to print this ?
		}
		go func() {
			// Send telemetry
			tracing.SpanWrapFunc("navigation_menu", tracing.KeyboardOptions(lk.metrics),
				func(ctx context.Context) error {
					return nil
				})(ctx)
		}()
		ShowCursor()
		// will notify main thread to kill and will handle gracefully
		lk.signalChannel <- syscall.SIGINT
	case keyboard.KeyEnter:
		lk.PrintEnter()
	case keyboard.KeyCtrlL:
		lk.CleanTerminal()
	}
}

func allocateSpace(lines int) {
	for i := 0; i < lines; i++ {
		ClearLine()
		NewLine()
		MoveCursorX(0)
	}
}

func isOverflow(s string) bool {
	return lenAnsi(s) > goterm.Width()
}

func linesOffset(s string) int {
	return int(math.Floor(float64(lenAnsi(s)) / float64(goterm.Width())))
}

func shortcutKeyColor(key string) string {
	foreground := "38;2"
	black := "0;0;0"
	background := "48;2"
	white := "255;255;255"
	bold := "1"
	return ansiColor(foreground+";"+black+";"+background+";"+white+";"+bold, key)
}
