package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

// Terminal a wrapper around a Cobra command, with extra methods
// to display messages.
type Terminal interface {
	InOrStdin() io.Reader
	OutOrStdout() io.Writer
	Debugf(msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Errorf(err error, msg string, args ...interface{})
	Fatalf(err error, msg string, args ...interface{})
	PromptBoolf(msg string, args ...interface{}) bool
	AddPreFatalExitHook(func())
}

// New returns a new terminal with the given funcs to
// access the `in` reader and `out` writer
func New(in func() io.Reader, out func() io.Writer, verbose bool) Terminal {
	return &DefaultTerminal{
		in:      in,
		out:     out,
		verbose: verbose,
	}
}

// InOrStdin returns an `io.Reader` to read the user's input
func (t *DefaultTerminal) InOrStdin() io.Reader {
	return t.in()
}

// OutOrStdout returns an `io.Writer` to write messages in the console
func (t *DefaultTerminal) OutOrStdout() io.Writer {
	return t.out()
}

// DefaultTerminal a wrapper around a Cobra command, with extra methods
// to display messages.
type DefaultTerminal struct {
	in             func() io.Reader
	out            func() io.Writer
	fatalExitHooks []func()
	verbose        bool
}

// Debugf prints a message (if verbose was enabled)
func (t DefaultTerminal) Debugf(msg string, args ...interface{}) {
	if !t.verbose {
		return
	}
	if msg == "" {
		fmt.Fprintln(t.OutOrStdout(), "")
		return
	}
	fmt.Fprintln(t.OutOrStdout(), fmt.Sprintf(msg, args...))
}

// Infof displays a message with the default color
func (t DefaultTerminal) Infof(msg string, args ...interface{}) {
	if msg == "" {
		fmt.Fprintln(t.OutOrStdout(), "")
		return
	}
	fmt.Fprintln(t.OutOrStdout(), fmt.Sprintf(msg, args...))
}

// Errorf prints a message with the red color
func (t DefaultTerminal) Errorf(err error, msg string, args ...interface{}) {
	color.New(color.FgRed).Fprintln(t.OutOrStdout(), fmt.Sprintf("%s: %s", fmt.Sprintf(msg, args...), err.Error())) // nolint:errcheck
}

// Fatalf prints a message with the red color and exits the program with a `1` return code
func (t DefaultTerminal) Fatalf(err error, msg string, args ...interface{}) {
	defer os.Exit(1)
	t.Errorf(err, msg, args...)
	for _, hook := range t.fatalExitHooks {
		hook()
	}
}

// PromptBoolf prints a message and waits for the user's boolean response
func (t DefaultTerminal) PromptBoolf(msg string, args ...interface{}) bool {
	fmt.Fprintln(t.OutOrStdout(), fmt.Sprintf(msg, args...))
	t.InOrStdin()

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf(msg, args...),
		IsConfirm: true,
	}

	result, err := prompt.Run()

	if err != nil && !errors.Is(err, promptui.ErrAbort) {
		t.Errorf(err, "😳 Prompt failed")
		return false
	}
	return strings.ToLower(result) == "y"
}

func (t *DefaultTerminal) AddPreFatalExitHook(hook func()) {
	t.fatalExitHooks = append(t.fatalExitHooks, hook)
}
