package color

import (
	"os"
	"strings"
	"testing"
)

func TestBoldDisabled(t *testing.T) {
	Enabled = false
	if got := Bold("hello"); got != "hello" {
		t.Errorf("Bold disabled = %q, want %q", got, "hello")
	}
}

func TestRedDisabled(t *testing.T) {
	Enabled = false
	if got := Red("error"); got != "error" {
		t.Errorf("Red disabled = %q, want %q", got, "error")
	}
}

func TestAllColorsEnabled(t *testing.T) {
	Enabled = true
	tests := []struct {
		name string
		fn   func(string) string
		code string
		in   string
	}{
		{"Bold", Bold, "[1m", "bold"},
		{"Faint", Faint, "[2m", "faint"},
		{"Red", Red, "[31m", "red"},
		{"Green", Green, "[32m", "green"},
		{"Yellow", Yellow, "[33m", "yellow"},
		{"Blue", Blue, "[34m", "blue"},
		{"Magenta", Magenta, "[35m", "magenta"},
		{"Cyan", Cyan, "[36m", "cyan"},
		{"White", White, "[37m", "white"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.in)
			if !strings.HasPrefix(got, "\033"+tt.code) {
				t.Errorf("%s = %q, want prefix \\033%s", tt.name, got, tt.code)
			}
			if !strings.Contains(got, tt.in) {
				t.Errorf("%s = %q, should contain %q", tt.name, got, tt.in)
			}
			if !strings.HasSuffix(got, "\033[0m") {
				t.Errorf("%s = %q, should end with reset", tt.name, got)
			}
		})
	}
}

func TestDisable(t *testing.T) {
	Enabled = true
	Disable()
	if Enabled {
		t.Error("Disable() should set Enabled=false")
	}
	if got := Red("x"); got != "x" {
		t.Error("Red should be no-op after Disable")
	}
	// Restore for other tests.
	Enabled = false
}

func TestFaint(t *testing.T) {
	Enabled = false
	if got := Faint("dim"); got != "dim" {
		t.Errorf("Faint disabled = %q", got)
	}
}

func TestGreen(t *testing.T) {
	Enabled = false
	if got := Green("ok"); got != "ok" {
		t.Errorf("Green disabled = %q", got)
	}
}

func TestYellow(t *testing.T) {
	Enabled = false
	if got := Yellow("warn"); got != "warn" {
		t.Errorf("Yellow disabled = %q", got)
	}
}

func TestBlue(t *testing.T) {
	Enabled = false
	if got := Blue("info"); got != "info" {
		t.Errorf("Blue disabled = %q", got)
	}
}

func TestMagenta(t *testing.T) {
	Enabled = false
	if got := Magenta("m"); got != "m" {
		t.Errorf("Magenta disabled = %q", got)
	}
}

func TestCyan(t *testing.T) {
	Enabled = false
	if got := Cyan("c"); got != "c" {
		t.Errorf("Cyan disabled = %q", got)
	}
}

func TestWhite(t *testing.T) {
	Enabled = false
	if got := White("w"); got != "w" {
		t.Errorf("White disabled = %q", got)
	}
}

func TestDetectNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	Enabled = true
	detect()
	if Enabled {
		t.Error("detect() with NO_COLOR should disable")
	}
}

func TestDetectTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	prev := isTerminal
	t.Cleanup(func() { isTerminal = prev })

	isTerminal = func() bool { return true }
	Enabled = false
	detect()
	if !Enabled {
		t.Error("detect() should enable when stdout is a terminal")
	}

	isTerminal = func() bool { return false }
	Enabled = true
	detect()
	if Enabled {
		t.Error("detect() should disable when stdout is not a terminal")
	}
}

func TestInitOnce(t *testing.T) {
	// Init wraps once.Do(detect); calling it must not panic and is idempotent.
	Init()
	Init()
}
