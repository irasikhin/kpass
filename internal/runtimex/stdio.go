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

func PromptSecret(prompt string, confirm bool) (string, error) {
	if PromptHook != nil {
		return PromptHook(prompt, confirm)
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin is not a terminal")
	}
	fmt.Fprint(os.Stderr, prompt)
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if !confirm {
		return string(first), nil
	}
	fmt.Fprint(os.Stderr, "Repeat "+prompt)
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("Values do not match.")
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
