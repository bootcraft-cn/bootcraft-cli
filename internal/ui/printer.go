package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var (
	isTTY  bool
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
)

func init() {
	isTTY = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

func IsTTY() bool {
	return isTTY
}

func Success(msg string) {
	_, _ = green.Fprintln(os.Stderr, msg)
}

func Error(msg string) {
	_, _ = red.Fprintln(os.Stderr, msg)
}

func Warn(msg string) {
	_, _ = yellow.Fprintln(os.Stderr, msg)
}

func Info(msg string) {
	_, _ = cyan.Fprintln(os.Stderr, msg)
}

func Print(msg string) {
	fmt.Fprint(os.Stderr, msg)
}

func Println(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func Printf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func Confirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt+" ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

// Spinner shows an animated spinner on stderr while work is in progress.
// Call Stop() to clear it. No-op when not a TTY.
type Spinner struct {
	msg  string
	stop chan struct{}
	done chan struct{}
}

func NewSpinner(msg string) *Spinner {
	s := &Spinner{msg: msg, stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer close(s.done)
	frames := []string{"⠋", "⠙", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-s.stop:
			if isTTY {
				fmt.Fprintf(os.Stderr, "\r\033[K")
			}
			return
		default:
			if isTTY {
				fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], s.msg)
			}
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}
