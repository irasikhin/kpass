package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/irasikhin/kpass/internal/clip"
)

// runClearClipboard is the hidden child-process entry: kpass spawns itself
// as `kpass __clear-clipboard <timeout-seconds>` with the secret on stdin,
// detaches, and the parent returns. The child sleeps `timeout` seconds,
// reads the current clipboard, and clears it only if it still matches the
// secret we put there (so a later `kpass copy` of a different secret won't
// be wiped).
//
// Returns the process exit code; stdout/stderr are ignored (the child has
// been detached from the parent's terminal).
func runClearClipboard(argv []string) int {
	if len(argv) < 2 {
		return 1
	}
	timeout, err := strconv.Atoi(argv[1])
	if err != nil || timeout <= 0 {
		return 1
	}
	secret, err := io.ReadAll(os.Stdin)
	if err != nil || len(secret) == 0 {
		return 1
	}
	time.Sleep(time.Duration(timeout) * time.Second)
	current, err := clip.Read()
	if err != nil {
		return 1
	}
	if current != string(secret) {
		// User has put something else on the clipboard since we wrote it;
		// leave it alone.
		return 0
	}
	if err := clip.Clear(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
