package formatter

import (
	"fmt"

	"github.com/buger/goterm"
)

type LogKeyboard struct {
	// mutex   sync.Mutex
	// message string
	err     error
	started bool
}

var KeyboardInfo = LogKeyboard{}
var errorColor = "\x1b[1;33m"

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
	lk.err = fmt.Errorf(err)
}
func (lk *LogKeyboard) Error(err error) {
	lk.err = err
}

// This avoids incorrect printing at the end of the terminal
func (lk *LogKeyboard) createBuffer() {
	fmt.Print("\012") // new line
	fmt.Print("\012")
	fmt.Print("\033[2A") // go back 3 lines
}

func (lk *LogKeyboard) printInfo() {
	height := goterm.Height()
	fmt.Print("\0337") // save cursor position
	if lk.err != nil {
		fmt.Printf("\033[%d;0H", height-1) // Move to before last line
		fmt.Printf("\033[K" + errorColor + "[Error]   " + lk.err.Error())
	}
	fmt.Printf("\033[%d;0H", height) // Move to last line
	// clear line
	fmt.Print("\033[K" + navColor("  >> [CTRL+G] open project in Docker Desktop [$] get more features"))
	fmt.Print("\0338") // restore cursor position
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
