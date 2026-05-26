package runtimex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"foo", "foo"},
		{"/foo/bar", "foo/bar"},
		{"foo/bar/", "foo/bar"},
		{"/foo/bar/", "foo/bar"},
		{"  /a/b/  ", "a/b"},
	}
	for _, tt := range tests {
		got := NormalizePath(tt.in)
		if got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}

	tests := []struct{ in, want string }{
		{"", ""},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
	}
	for _, tt := range tests {
		got := ExpandPath(tt.in)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo/bar", []string{"foo", "bar"}},
		{"/foo/bar/", []string{"foo", "bar"}},
		{"a/b/c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		got := SplitPath(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("SplitPath(%q) = %v (len=%d), want %v (len=%d)", tt.in, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("SplitPath(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"foo"}, "foo"},
		{[]string{"foo", "bar"}, "foo/bar"},
		{[]string{"", "foo"}, "foo"},
		{[]string{"foo", "", "bar"}, "foo/bar"},
	}
	for _, tt := range tests {
		got := JoinPath(tt.in)
		if got != tt.want {
			t.Errorf("JoinPath(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRoundtrip(t *testing.T) {
	paths := []string{"foo", "foo/bar", "a/b/c/d", "work/email"}
	for _, p := range paths {
		parts := SplitPath(p)
		joined := JoinPath(parts)
		if joined != p {
			t.Errorf("roundtrip(%q): Split+Join = %q", p, joined)
		}
	}
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("KPASS_CONFIG", "")
	got := ConfigFilePath("")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config/kpass/config.toml")
	if got != expected {
		t.Errorf("default = %q, want %q", got, expected)
	}

	got = ConfigFilePath("/custom/path.toml")
	if got != "/custom/path.toml" {
		t.Errorf("explicit = %q, want /custom/path.toml", got)
	}

	t.Setenv("KPASS_CONFIG", "~/env.toml")
	got = ConfigFilePath("")
	if got != filepath.Join(home, "env.toml") {
		t.Errorf("env = %q, want %q", got, filepath.Join(home, "env.toml"))
	}
}
