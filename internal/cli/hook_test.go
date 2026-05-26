package cli

import (
	"testing"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// setPromptHook installs a runtimex.PromptHook that returns the given
// password and increments callCount. Restored on t.Cleanup.
func setPromptHook(t *testing.T, password string, callCount *int) bool {
	t.Helper()
	prev := runtimex.PromptHook
	runtimex.PromptHook = func(prompt string, confirm bool) (string, error) {
		*callCount++
		return password, nil
	}
	t.Cleanup(func() { runtimex.PromptHook = prev })
	return true
}

func setPromptHookFail(t *testing.T) {
	t.Helper()
	prev := runtimex.PromptHook
	runtimex.PromptHook = func(prompt string, confirm bool) (string, error) {
		t.Fatal("prompt should not be invoked")
		return "", nil
	}
	t.Cleanup(func() { runtimex.PromptHook = prev })
}
