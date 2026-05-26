package cli

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-V"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{flag}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
			out := stdout.String()
			if !strings.HasPrefix(out, "kpass ") {
				t.Fatalf("want kpass-prefix, got %q", out)
			}
			if !strings.Contains(out, runtime.GOOS) || !strings.Contains(out, runtime.GOARCH) {
				t.Fatalf("expected GOOS/GOARCH in version line, got %q", out)
			}
		})
	}
}

func TestRootHelpListsAllCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ls", "search", "get", "copy", "attach", "pick",
		"insert", "edit", "generate", "remove", "move",
		"duplicate", "mkdir", "merge", "doctor", "db",
		"--config", "--database", "--password-file",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("root help missing %q. got:\n%s", want, out)
		}
	}
}

func TestPerCommandHelpRenders(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"ls", "--help"}, []string{"List entries as a tree", "--flat", "--groups"}},
		{[]string{"get", "--help"}, []string{"Show one entry", "--field"}},
		{[]string{"copy", "--help"}, []string{"Copy a field", "--field"}},
		{[]string{"insert", "--help"}, []string{"Create a new entry", "--username", "--generate"}},
		{[]string{"attach", "--help"}, []string{"add", "remove", "extract", "ls"}},
		{[]string{"db", "--help"}, []string{"add", "rm", "default", "ls"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
			out := stdout.String()
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("help for %v missing %q. got:\n%s", tc.args, want, out)
				}
			}
		})
	}
}

func TestShortAliasesRemoved(t *testing.T) {
	// All single-letter and short command aliases (s, g, c, i, e, rm, mv, dup)
	// were removed. They should now produce an unknown-command error.
	f := newFixture(t)
	for _, alias := range []string{"s", "g", "c", "i", "e", "rm", "mv", "dup"} {
		_, stderr, code := f.runCLI(alias)
		if code != 1 {
			t.Fatalf("alias %q: expected exit 1, got %d, stderr=%q", alias, code, stderr)
		}
		if !strings.Contains(stderr, "invalid choice:") {
			t.Fatalf("alias %q: stderr missing 'invalid choice': %q", alias, stderr)
		}
	}
}

func TestCloneCommandRemoved(t *testing.T) {
	// `clone` was renamed to `duplicate` in v0.4.0 (still accepted as an
	// alias there) and removed entirely in v0.5.0. Users land in the
	// unknown-command path.
	f := newFixture(t)
	_, stderr, code := f.runCLI("clone", "internet/email", "x/y")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d, stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "invalid choice: 'clone'") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestCpAliasRemoved(t *testing.T) {
	// `cp` was confusing (copy vs clone semantics); it was removed in v0.4.0.
	// The unknown-command path should suggest the closest live name.
	f := newFixture(t)
	_, stderr, code := f.runCLI("cp", "internet/email", "x/y")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d, stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "invalid choice: 'cp'") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestNoArgsShowsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("expected help output, got:\n%s", out)
	}
}

func TestUnknownCommandSuggestion(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("getz")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "Did you mean 'get'") {
		t.Fatalf("stderr=%q", stderr)
	}
}
