package tree

import (
	"strings"
	"testing"
)

func TestRender_Empty(t *testing.T) {
	out := Render(nil, "Root")
	if !strings.Contains(out, "Root") {
		t.Errorf("Render empty: %q", out)
	}
}

func TestRender_SingleEntry(t *testing.T) {
	out := Render([]string{"email"}, "Store")
	if !strings.Contains(out, "email") {
		t.Errorf("missing email: %q", out)
	}
}

func TestRender_Nested(t *testing.T) {
	out := Render([]string{"work/email", "work/chat", "personal/bank"}, "Vault")
	if !strings.Contains(out, "work") {
		t.Error("missing work group")
	}
	if !strings.Contains(out, "email") {
		t.Error("missing email entry")
	}
	if !strings.Contains(out, "personal") {
		t.Error("missing personal group")
	}
	if !strings.Contains(out, "bank") {
		t.Error("missing bank entry")
	}
}

func TestRender_Sorted(t *testing.T) {
	out := Render([]string{"zebra", "alpha", "beta"}, "Root")
	// Should be sorted case-insensitively: alpha, beta, zebra
	alphaIdx := strings.Index(out, "alpha")
	betaIdx := strings.Index(out, "beta")
	zebraIdx := strings.Index(out, "zebra")
	if alphaIdx == -1 || betaIdx == -1 || zebraIdx == -1 {
		t.Fatal("missing entries")
	}
	if alphaIdx > betaIdx || betaIdx > zebraIdx {
		t.Errorf("not sorted: alpha=%d beta=%d zebra=%d", alphaIdx, betaIdx, zebraIdx)
	}
}

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
