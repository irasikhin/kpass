package tree

import (
	"strings"
	"testing"
)

func TestRenderRich_Indicators(t *testing.T) {
	entries := []*EntryInfo{
		{Path: "work/email", HasPassword: true, HasURL: true, HasOTP: true},
		{Path: "simple", HasPassword: true},
	}
	out := RenderRich(entries, "Vault", 0)
	if !strings.Contains(out, "🔑") {
		t.Error("missing password indicator")
	}
	if !strings.Contains(out, "🔗") {
		t.Error("missing URL indicator")
	}
	if !strings.Contains(out, "⏱") {
		t.Error("missing OTP indicator")
	}
}

func TestRenderRich_GroupCounts(t *testing.T) {
	entries := []*EntryInfo{
		{Path: "work/email"},
		{Path: "work/chat"},
		{Path: "personal/bank"},
	}
	out := RenderRich(entries, "Vault", 0)
	if !strings.Contains(out, "(2)") {
		t.Errorf("expected (2) count for work group in: %q", out)
	}
	if !strings.Contains(out, "(1)") {
		t.Errorf("expected (1) count for personal group in: %q", out)
	}
}

func TestRenderRich_Depth(t *testing.T) {
	entries := []*EntryInfo{
		{Path: "a/b/c/entry"},
	}
	out := RenderRich(entries, "Vault", 1)
	// At depth 1, we should see "a" group but not "b" children.
	if !strings.Contains(out, "a") {
		t.Error("missing group a")
	}
	// b should not appear beyond depth 1.
	if strings.Count(out, "b") > 0 {
		t.Error("group b should not appear at depth 1")
	}
}

func TestRenderLong_Table(t *testing.T) {
	entries := []*EntryInfo{
		{Path: "work/email", Username: "alice", URL: "https://example.com", HasOTP: true, AttachCount: 0, Tags: ""},
		{Path: "personal/bank", Username: "bob", URL: "https://bank.com", HasOTP: false, AttachCount: 2, Tags: "finance"},
	}
	out := RenderLong(entries)
	if !strings.Contains(out, "PATH") || !strings.Contains(out, "USER") || !strings.Contains(out, "URL") {
		t.Error("missing table headers")
	}
	if !strings.Contains(out, "work/email") {
		t.Error("missing work/email row")
	}
	if !strings.Contains(out, "personal/bank") {
		t.Error("missing personal/bank row")
	}
	if !strings.Contains(out, "alice") {
		t.Error("missing username alice")
	}
	if !strings.Contains(out, "finance") {
		t.Error("missing finance tag")
	}
}

func TestEntryInfo_Indicators(t *testing.T) {
	e := &EntryInfo{HasPassword: true, HasURL: true, HasNotes: true, HasOTP: true, AttachCount: 3}
	ind := e.Indicators()
	if !strings.Contains(ind, "🔑") {
		t.Error("missing 🔑")
	}
	if !strings.Contains(ind, "🔗") {
		t.Error("missing 🔗")
	}
	if !strings.Contains(ind, "📝") {
		t.Error("missing 📝")
	}
	if !strings.Contains(ind, "⏱") {
		t.Error("missing ⏱")
	}
	if !strings.Contains(ind, "📎3") {
		t.Error("missing 📎3")
	}
}

func TestEntryInfo_Indicators_Empty(t *testing.T) {
	e := &EntryInfo{}
	if e.Indicators() != "" {
		t.Errorf("empty indicators = %q", e.Indicators())
	}
}

func TestEntryInfo_Indicators_Suffix(t *testing.T) {
	e := &EntryInfo{Suffix: "(stale)"}
	got := e.Indicators()
	if !strings.Contains(got, "(stale)") {
		t.Errorf("suffix missing in %q", got)
	}
}

func TestRenderLong_MinColumnDefaults(t *testing.T) {
	// Short fields → min widths kick in; no OTP/attach exercises the "—" placeholders.
	entries := []*EntryInfo{{Path: "a", Username: "u", URL: "x"}}
	out := RenderLong(entries)
	if !strings.Contains(out, "  —") {
		t.Errorf("expected dash placeholder for missing TOTP/attach in %q", out)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Errorf("truncate long = %q", got)
	}
}
