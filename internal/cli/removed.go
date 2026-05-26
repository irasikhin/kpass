package cli

func runClearClipboard(argv []string) int {
	// With the pure-Go clipboard, auto-clear is handled by a goroutine
	// inside clip.WriteWithAutoClear. This function is kept for backwards
	// compatibility with any existing detached processes.
	return 0
}
