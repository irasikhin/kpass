package runtimex

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func resetStdio(t *testing.T) {
	t.Helper()
	origPrompt := PromptHook
	origRead := ReadStdinHook
	origIsTerm := isTerminalFn
	origReadPw := readPasswordFn
	origFd := stdinFdFn
	origWriter := promptWriter
	t.Cleanup(func() {
		PromptHook = origPrompt
		ReadStdinHook = origRead
		isTerminalFn = origIsTerm
		readPasswordFn = origReadPw
		stdinFdFn = origFd
		promptWriter = origWriter
	})
}

func TestPromptSecret_HookOverride(t *testing.T) {
	resetStdio(t)
	var capturedPrompt string
	var capturedConfirm bool
	PromptHook = func(prompt string, confirm bool) (string, error) {
		capturedPrompt = prompt
		capturedConfirm = confirm
		return "via-hook", nil
	}
	got, err := PromptSecret("Master: ", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "via-hook" {
		t.Errorf("value = %q, want via-hook", got)
	}
	if capturedPrompt != "Master: " || !capturedConfirm {
		t.Errorf("hook saw prompt=%q confirm=%t", capturedPrompt, capturedConfirm)
	}
}

func TestPromptSecret_NotATerminal(t *testing.T) {
	resetStdio(t)
	isTerminalFn = func(int) bool { return false }
	stdinFdFn = func() int { return 0 }

	if _, err := PromptSecret("X: ", false); err == nil || !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("expected not-a-terminal error, got %v", err)
	}
}

func TestPromptSecret_SinglePromptSuccess(t *testing.T) {
	resetStdio(t)
	var buf bytes.Buffer
	isTerminalFn = func(int) bool { return true }
	stdinFdFn = func() int { return 0 }
	readPasswordFn = func(int) ([]byte, error) { return []byte("topsecret"), nil }
	promptWriter = &buf

	got, err := PromptSecret("Password: ", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "topsecret" {
		t.Errorf("value = %q, want topsecret", got)
	}
	if !strings.Contains(buf.String(), "Password: ") {
		t.Errorf("prompt not written to writer: %q", buf.String())
	}
}

func TestPromptSecret_ReadError(t *testing.T) {
	resetStdio(t)
	isTerminalFn = func(int) bool { return true }
	stdinFdFn = func() int { return 0 }
	readPasswordFn = func(int) ([]byte, error) { return nil, errors.New("io fail") }
	promptWriter = io.Discard

	if _, err := PromptSecret("X: ", false); err == nil || !strings.Contains(err.Error(), "io fail") {
		t.Errorf("expected io fail, got %v", err)
	}
}

func TestPromptSecret_ConfirmMatch(t *testing.T) {
	resetStdio(t)
	calls := 0
	isTerminalFn = func(int) bool { return true }
	stdinFdFn = func() int { return 0 }
	readPasswordFn = func(int) ([]byte, error) {
		calls++
		return []byte("samesame"), nil
	}
	promptWriter = io.Discard

	got, err := PromptSecret("New: ", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "samesame" {
		t.Errorf("value = %q", got)
	}
	if calls != 2 {
		t.Errorf("read calls = %d, want 2 (initial+confirm)", calls)
	}
}

func TestPromptSecret_ConfirmMismatch(t *testing.T) {
	resetStdio(t)
	calls := 0
	isTerminalFn = func(int) bool { return true }
	stdinFdFn = func() int { return 0 }
	readPasswordFn = func(int) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("first"), nil
		}
		return []byte("second"), nil
	}
	promptWriter = io.Discard

	if _, err := PromptSecret("X: ", true); err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Errorf("expected mismatch error, got %v", err)
	}
}

func TestPromptSecret_ConfirmReadError(t *testing.T) {
	resetStdio(t)
	calls := 0
	isTerminalFn = func(int) bool { return true }
	stdinFdFn = func() int { return 0 }
	readPasswordFn = func(int) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("ok"), nil
		}
		return nil, errors.New("confirm fail")
	}
	promptWriter = io.Discard

	if _, err := PromptSecret("X: ", true); err == nil || !strings.Contains(err.Error(), "confirm fail") {
		t.Errorf("expected confirm fail, got %v", err)
	}
}

func TestReadSecretFromStdin_Hook(t *testing.T) {
	resetStdio(t)
	ReadStdinHook = func(in io.Reader) (string, error) { return "from-hook", nil }
	got, err := ReadSecretFromStdin(strings.NewReader("ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-hook" {
		t.Errorf("value = %q", got)
	}
}

func TestReadSecretFromStdin_TrimsTrailingNewline(t *testing.T) {
	resetStdio(t)
	got, err := ReadSecretFromStdin(strings.NewReader("secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Errorf("value = %q, want %q", got, "secret")
	}
}

func TestReadSecretFromStdin_NoNewline(t *testing.T) {
	resetStdio(t)
	got, err := ReadSecretFromStdin(strings.NewReader("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Errorf("value = %q", got)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }

func TestReadSecretFromStdin_ReaderError(t *testing.T) {
	resetStdio(t)
	if _, err := ReadSecretFromStdin(errReader{}); err == nil {
		t.Error("expected reader error to bubble")
	}
}
