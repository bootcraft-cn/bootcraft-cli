package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var (
	isTTY   bool
	green   = color.New(color.FgGreen)
	red     = color.New(color.FgRed)
	yellow  = color.New(color.FgYellow)
	cyan    = color.New(color.FgCyan)
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
	green.Fprintln(os.Stderr, msg)
}

func Error(msg string) {
	red.Fprintln(os.Stderr, msg)
}

func Warn(msg string) {
	yellow.Fprintln(os.Stderr, msg)
}

func Info(msg string) {
	cyan.Fprintln(os.Stderr, msg)
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
