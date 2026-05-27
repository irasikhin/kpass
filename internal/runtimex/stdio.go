package runtimex

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PromptHook lets tests override password prompts.
var PromptHook func(prompt string, confirm bool) (string, error)

// ReadStdinHook lets tests override stdin reads.
var ReadStdinHook func(in io.Reader) (string, error)

// Seams for term/stdio I/O so unit tests can avoid a real TTY.
var (
	stdinFdFn                = func() int { return int(os.Stdin.Fd()) }
	isTerminalFn             = term.IsTerminal
	readPasswordFn           = term.ReadPassword
	promptWriter   io.Writer = os.Stderr
)

func PromptSecret(prompt string, confirm bool) (string, error) {
	if PromptHook != nil {
		return PromptHook(prompt, confirm)
	}
	fd := stdinFdFn()
	if !isTerminalFn(fd) {
		return "", errors.New("stdin is not a terminal")
	}
	fmt.Fprint(promptWriter, prompt)
	first, err := readPasswordFn(fd)
	fmt.Fprintln(promptWriter)
	if err != nil {
		return "", err
	}
	if !confirm {
		return string(first), nil
	}
	fmt.Fprint(promptWriter, "Repeat "+prompt)
	second, err := readPasswordFn(fd)
	fmt.Fprintln(promptWriter)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("values do not match")
	}
	return string(first), nil
}

func ReadSecretFromStdin(in io.Reader) (string, error) {
	if ReadStdinHook != nil {
		return ReadStdinHook(in)
	}
	data, err := io.ReadAll(bufio.NewReader(in))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}
