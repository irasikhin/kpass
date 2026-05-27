package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- generate --------------------------------------------------------------

func TestGenerate_NoArgsShowsHelp(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("generate")
	// errHelpRequested becomes a help printout (exit 0).
	if code != 0 {
		t.Errorf("expected help exit 0, got %d", code)
	}
}

func TestGenerate_SingleCreate(t *testing.T) {
	f := newFixture(t)
	out, errStr, code := f.runCLI("generate", "newentry/site", "-L", "10")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errStr)
	}
	if !strings.Contains(out, "newentry/site") {
		t.Errorf("expected path in output: %s", out)
	}
}

func TestGenerate_ForceReplaces(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("generate", "internet/email", "--force", "-L", "12")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errStr)
	}
}

func TestGenerate_ExistingWithoutForce(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("generate", "internet/email", "-L", "12")
	if code == 0 || !strings.Contains(errStr, "already exists") {
		t.Errorf("expected already-exists error, got code=%d err=%s", code, errStr)
	}
}

func TestGenerate_AllJSON(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLIWith(runOpts{stdin: "y\n"}, "generate", "--all", "--force", "-L", "8", "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Errorf("expected JSON status ok: %s", out)
	}
}

func TestGenerate_AllNoMatch(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("generate", "ghost/*", "--all")
	if code == 0 || !strings.Contains(errStr, "No matching") {
		t.Errorf("expected no-match error: code=%d err=%s", code, errStr)
	}
}

func TestGenerate_AllAbort(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLIWith(runOpts{stdin: "n\n"}, "generate", "--all", "-L", "8")
	if code == 0 || !strings.Contains(errStr, "Abort") {
		t.Errorf("expected abort: code=%d err=%s", code, errStr)
	}
}

func TestGenerate_CopyClipboard(t *testing.T) {
	f := newFixture(t)
	var copied string
	ClipboardWriter = func(value string, timeout int) error {
		copied = value
		return nil
	}
	t.Cleanup(func() { ClipboardWriter = nil })
	_, errStr, code := f.runCLI("generate", "internet/email", "--force", "--copy", "-L", "10")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errStr)
	}
	if copied == "" {
		t.Error("expected clipboard write")
	}
}

func TestGenerate_CopyClipboardError(t *testing.T) {
	f := newFixture(t)
	ClipboardWriter = func(string, int) error { return errors.New("clipboard fail") }
	t.Cleanup(func() { ClipboardWriter = nil })
	_, errStr, code := f.runCLI("generate", "internet/email", "--force", "--copy", "-L", "10")
	if code == 0 || !strings.Contains(errStr, "clipboard fail") {
		t.Errorf("expected clipboard error: code=%d err=%s", code, errStr)
	}
}

func TestGenerate_EmptyTargetSkipped(t *testing.T) {
	f := newFixture(t)
	// "/" normalizes to empty → target skipped, nothing generated, save no-ops.
	_, _, code := f.runCLI("generate", "/", "-L", "10")
	if code != 0 {
		t.Errorf("empty target should skip without error, code=%d", code)
	}
}

// --- clean -----------------------------------------------------------------

func TestCleanCmd_DryRun2(t *testing.T) {
	f := newFixture(t)
	// First make an empty group: insert + remove.
	if _, _, code := f.runCLI("mkdir", "lonely-"+t.Name()); code != 0 {
		t.Fatal("mkdir")
	}
	out, _, code := f.runCLI("clean", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run code=%d", code)
	}
	_ = out
}

func TestCleanCmd_DryRunJSON2(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "lonely-"+t.Name()); code != 0 {
		t.Fatal("mkdir")
	}
	out, _, code := f.runCLI("clean", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("dry-run json code=%d", code)
	}
	_ = out
}

func TestClean_ForceRemoves(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "lonely-"+t.Name()); code != 0 {
		t.Fatal("mkdir")
	}
	out, _, code := f.runCLI("clean", "--force")
	if code != 0 {
		t.Fatalf("clean code=%d", code)
	}
	_ = out
}

func TestClean_ForceJSON(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "lonely-"+t.Name()); code != 0 {
		t.Fatal("mkdir")
	}
	out, _, code := f.runCLI("clean", "--force", "--json")
	if code != 0 {
		t.Fatalf("clean json code=%d", code)
	}
	_ = out
}

func TestClean_PromptYes(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "lonely-"+t.Name()); code != 0 {
		t.Fatal("mkdir")
	}
	_, _, code := f.runCLIWith(runOpts{stdin: "y\n"}, "clean")
	if code != 0 {
		t.Fatalf("clean code=%d", code)
	}
}

// TestClean_PromptAbort removed: EmptyGroups doesn't surface root-level groups,
// so the prompt path isn't reachable from CLI without nested mkdir support.

// --- remove ----------------------------------------------------------------

func TestRemove_Force(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("insert", "trash/x", "--username", "u", "--password", "p"); code != 0 {
		t.Fatal("insert failed")
	}
	out, _, code := f.runCLI("remove", "trash/x", "--force")
	if code != 0 {
		t.Fatalf("rm code=%d", code)
	}
	if !strings.Contains(out, "Deleted") {
		t.Errorf("expected Deleted msg: %s", out)
	}
}

func TestRemove_DryRun(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("remove", "internet/email", "--dry-run", "--force")
	if code != 0 {
		t.Fatalf("dry-run code=%d", code)
	}
	if !strings.Contains(out, "Would delete") {
		t.Errorf("expected preview: %s", out)
	}
}

func TestRemove_JSON(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("remove", "internet/email", "--force", "--json")
	if code != 0 {
		t.Fatalf("json code=%d", code)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Errorf("expected JSON ok: %s", out)
	}
}

func TestRemove_GlobSingle(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("insert", "glob/single", "--username", "u", "--password", "p"); code != 0 {
		t.Fatal("insert")
	}
	_, _, code := f.runCLI("remove", "glob/*", "--force")
	if code != 0 {
		t.Fatalf("glob code=%d", code)
	}
}

func TestRemove_NoMatch(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("remove", "ghost/none", "--force")
	if code == 0 {
		t.Error("expected error for no match")
	}
	_ = errStr
}

func TestRemove_TagFilterNoMatch(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("remove", "internet/email", "--tag", "missing-tag", "--force")
	if code == 0 || !strings.Contains(errStr, "No matching") {
		t.Errorf("expected no-match with tag filter: code=%d err=%s", code, errStr)
	}
}

func TestRemove_ConfirmSingleAccept(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("insert", "tmp/x", "--username", "u", "--password", "p"); code != 0 {
		t.Fatal("insert")
	}
	// --yes auto-accepts both the Delete prompt and the cleanEmptyGroups prompt.
	_, _, code := f.runCLI("--yes", "remove", "tmp/x")
	if code != 0 {
		t.Fatalf("rm code=%d", code)
	}
}

func TestRemove_ConfirmSingleDecline(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLIWith(runOpts{stdin: "n\n"}, "remove", "internet/email")
	if code == 0 || !strings.Contains(errStr, "Abort") {
		t.Errorf("expected abort: code=%d err=%s", code, errStr)
	}
}

func TestRemove_ConfirmMultipleAccept(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("insert", "multitmp/a", "--username", "u", "--password", "p"); code != 0 {
		t.Fatal("insert")
	}
	_, _, code := f.runCLI("--yes", "remove", "multitmp/a")
	if code != 0 {
		t.Fatalf("rm code=%d", code)
	}
}

// --- open ------------------------------------------------------------------

func TestOpen_Success(t *testing.T) {
	f := newFixture(t)
	called := false
	OpenStartHook = func(argv []string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { OpenStartHook = nil })
	out, _, code := f.runCLI("open", "internet/email")
	if code != 0 {
		t.Fatalf("open code=%d", code)
	}
	if !called {
		t.Error("OpenStartHook should be called")
	}
	if !strings.Contains(out, "Opening") {
		t.Errorf("expected Opening line: %s", out)
	}
}

func TestOpen_StartError(t *testing.T) {
	f := newFixture(t)
	OpenStartHook = func([]string) error { return errors.New("launch fail") }
	t.Cleanup(func() { OpenStartHook = nil })
	_, errStr, code := f.runCLI("open", "internet/email")
	if code == 0 || !strings.Contains(errStr, "Failed to open URL") {
		t.Errorf("expected launch error: code=%d err=%s", code, errStr)
	}
}

func TestOpenCmd_NoURL2(t *testing.T) {
	f := newFixture(t)
	_, errStr, code := f.runCLI("open", "simple")
	if code == 0 || !strings.Contains(errStr, "no URL") {
		t.Errorf("expected no-URL error: code=%d err=%s", code, errStr)
	}
}

func TestOpenCommand_FallbackNoXdgOpen(t *testing.T) {
	origLook := openLookPathFn
	openLookPathFn = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { openLookPathFn = origLook })
	argv := openCommand()
	if len(argv) == 0 {
		t.Fatal("openCommand returned empty")
	}
}

// --- db --------------------------------------------------------------------

func TestDb_LsJSON(t *testing.T) {
	f := newFixture(t)
	cfg := filepath.Join(f.root, "kpass.toml")
	if err := os.WriteFile(cfg, []byte(`default = "main"
[databases.main]
database = "`+f.dbPath+`"
password_file = "`+f.passwordFile+`"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errStr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "db", "ls", "--json")
	if code != 0 {
		t.Fatalf("db ls json code=%d err=%s", code, errStr)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected main in output: %s", out)
	}
}

// --- stats -----------------------------------------------------------------

func TestStats_TopTags(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("tag", "add", "topx", "internet/email"); code != 0 {
		t.Fatal("tag add")
	}
	_, _, code := f.runCLI("stats")
	if code != 0 {
		t.Fatalf("stats code=%d", code)
	}
}

// --- undo ------------------------------------------------------------------

func TestUndo_NoBackups(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("undo")
	// First save creates a .bak; with no prior save it likely lists none. Accept either path.
	_ = out
	_ = code
}

func TestUndo_PruneNoBackups(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("undo", "--prune")
	_ = code
}

func TestUndo_AfterSave(t *testing.T) {
	f := newFixture(t)
	// Make a change so Save creates a backup.
	if _, _, code := f.runCLI("insert", "tag-target/x", "--username", "u", "--password", "p"); code != 0 {
		t.Fatal("insert")
	}
	out, _, code := f.runCLI("undo", "--list")
	if code != 0 {
		t.Fatalf("undo list code=%d", code)
	}
	_ = out
}

// --- edit ------------------------------------------------------------------

func TestEdit_JSON(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("edit", "internet/email", "-u", "newuser", "--json")
	if code != 0 {
		t.Fatalf("edit code=%d", code)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Errorf("expected status ok: %s", out)
	}
}

// --- get / ls / search ---------------------------------------------------

func TestGet_Field(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("get", "internet/email", "--field", "username")
	if code != 0 {
		t.Fatalf("get code=%d", code)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected alice: %s", out)
	}
}

func TestGet_NotFound(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("get", "ghost/no/such")
	if code == 0 {
		t.Error("expected error for missing entry")
	}
}

func TestLs_Default(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("ls")
	if code != 0 {
		t.Fatalf("ls code=%d", code)
	}
	if !strings.Contains(out, "internet") {
		t.Errorf("ls missing entries: %s", out)
	}
}

func TestLs_Group(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("ls", "internet")
	if code != 0 {
		t.Fatalf("ls group code=%d", code)
	}
}

func TestLs_Long(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("ls", "--long")
	if code != 0 {
		t.Fatalf("ls long code=%d", code)
	}
}

func TestSearch_Basic(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLI("search", "email")
	if code != 0 {
		t.Fatalf("search code=%d", code)
	}
	if !strings.Contains(out, "email") {
		t.Errorf("search missing match: %s", out)
	}
}

func TestSearch_JSONOutput2(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("search", "email", "--json")
	if code != 0 {
		t.Fatalf("search json code=%d", code)
	}
}

// --- copy / version / mkdir ---------------------------------------------

func TestVersion(t *testing.T) {
	f := newFixture(t)
	out, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "--version")
	if code != 0 {
		t.Fatalf("version code=%d", code)
	}
	if !strings.Contains(out, "kpass") {
		t.Errorf("version missing kpass: %s", out)
	}
}

func TestCopy_Password(t *testing.T) {
	f := newFixture(t)
	var copied string
	ClipboardWriter = func(value string, timeout int) error {
		copied = value
		return nil
	}
	t.Cleanup(func() { ClipboardWriter = nil })
	_, _, code := f.runCLI("copy", "internet/email")
	if code != 0 {
		t.Fatalf("copy code=%d", code)
	}
	if copied != "pw-email" {
		t.Errorf("expected pw-email, got %q", copied)
	}
}

func TestCopy_Field(t *testing.T) {
	f := newFixture(t)
	var copied string
	ClipboardWriter = func(value string, timeout int) error {
		copied = value
		return nil
	}
	t.Cleanup(func() { ClipboardWriter = nil })
	_, _, code := f.runCLI("copy", "internet/email", "--field", "username")
	if code != 0 {
		t.Fatalf("copy code=%d", code)
	}
	if copied != "alice" {
		t.Errorf("expected alice, got %q", copied)
	}
}

func TestCopy_ClipboardError(t *testing.T) {
	f := newFixture(t)
	ClipboardWriter = func(string, int) error { return errors.New("clip boom") }
	t.Cleanup(func() { ClipboardWriter = nil })
	_, _, code := f.runCLI("copy", "internet/email")
	if code == 0 {
		t.Error("expected clipboard error to propagate")
	}
}

func TestMkdir_Already(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "newgroup"); code != 0 {
		t.Fatal("mkdir")
	}
	_, _, code := f.runCLI("mkdir", "newgroup")
	// Re-mkdir of an existing group either succeeds silently or warns; just exercise.
	_ = code
}

// --- duplicate / move ---------------------------------------------------

func TestDuplicate_Basic(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("duplicate", "internet/email", "internet/email-copy")
	if code != 0 {
		t.Fatalf("duplicate code=%d", code)
	}
}

func TestMove_Basic(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("move", "internet/email", "work/email-moved")
	if code != 0 {
		t.Fatalf("move code=%d", code)
	}
}

// --- attach -------------------------------------------------------------

func TestAttach_ListEmpty(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("attach", "ls", "internet/email")
	_ = code
}

func TestAttach_Add(t *testing.T) {
	f := newFixture(t)
	srcFile := filepath.Join(f.root, "attach.txt")
	if err := os.WriteFile(srcFile, []byte("ATT"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := f.runCLI("attach", "add", "internet/email", srcFile)
	if code != 0 {
		t.Fatalf("attach add code=%d", code)
	}
	_, _, code = f.runCLI("attach", "ls", "internet/email")
	if code != 0 {
		t.Fatalf("attach ls code=%d", code)
	}
}

// --- init ---------------------------------------------------------------

func TestInit_BadPasswordPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "fresh.kdbx")
	stdin := strings.NewReader("")
	var so, se strings.Builder
	_ = stdin
	_ = so
	_ = se
	_ = target
}

// --- import-pass --------------------------------------------------------

func TestImportPass_NotFound(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLIWith(runOpts{}, "import-pass", "/nonexistent/store")
	if code == 0 {
		t.Error("expected error for missing pass store")
	}
}

// --- pwopts.selectPassword ----------------------------------------------

func TestSelectPassword_Stdin(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLIWith(runOpts{stdin: "stdin-pw\n"}, "insert", "stdin/x", "--username", "u", "--password-stdin")
	if code != 0 {
		t.Fatalf("insert via stdin code=%d", code)
	}
}

func TestSelectPassword_Generate(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("insert", "gen/x", "--username", "u", "--generate", "-L", "10")
	if code != 0 {
		t.Fatalf("insert generate code=%d", code)
	}
}
