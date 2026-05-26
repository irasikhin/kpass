package cli

import "github.com/irasikhin/kpass/internal/clip"

// ClipboardWriter is the injection seam used by tests (replaces Python's
// `patch.object(KPASS, "copy_to_clipboard")`). When unset, calls real clip.
var ClipboardWriter func(value string, timeout int) error

func clipboardWrite(value string, timeout int) error {
	if ClipboardWriter != nil {
		return ClipboardWriter(value, timeout)
	}
	return clip.WriteWithAutoClear(value, timeout)
}
