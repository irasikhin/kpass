package db

import (
	"strings"
	"testing"
)

func TestAttachmentList_Sorted(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if err := e.AddAttachment("Banana.bin", []byte("two"), false); err != nil {
		t.Fatal(err)
	}
	if err := e.AddAttachment("apple.bin", []byte("three"), false); err != nil {
		t.Fatal(err)
	}
	got := e.AttachmentList()
	want := []string{"apple.bin", "Banana.bin", "doc.txt"}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if !strings.EqualFold(got[i], want[i]) {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAttachmentContent(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	got, err := e.AttachmentContent("doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ATTACHMENT-BODY" {
		t.Errorf("content = %q", got)
	}
}

func TestAttachmentContent_NotFound(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if _, err := e.AttachmentContent("ghost.txt"); err == nil {
		t.Error("expected not-found error")
	}
}

func TestAttachmentExists(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if !e.AttachmentExists("doc.txt") {
		t.Error("doc.txt should exist")
	}
	if e.AttachmentExists("ghost") {
		t.Error("ghost should not exist")
	}
}

func TestAddAttachment_DuplicateRefused(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	err := e.AddAttachment("doc.txt", []byte("dup"), false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected already-exists error, got %v", err)
	}
}

func TestAddAttachment_ForceReplaces(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if err := e.AddAttachment("doc.txt", []byte("UPDATED"), true); err != nil {
		t.Fatal(err)
	}
	got, err := e.AttachmentContent("doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "UPDATED" {
		t.Errorf("force-replace content = %q", got)
	}
	if list := e.AttachmentList(); len(list) != 1 {
		t.Errorf("force-replace should keep 1 attachment, got %v", list)
	}
}

func TestAddAttachment_New(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")
	if err := e.AddAttachment("note.txt", []byte("hello"), false); err != nil {
		t.Fatal(err)
	}
	if !e.AttachmentExists("note.txt") {
		t.Error("attachment not added")
	}
}

func TestRemoveAttachment(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if err := e.RemoveAttachment("doc.txt"); err != nil {
		t.Fatal(err)
	}
	if e.AttachmentExists("doc.txt") {
		t.Error("attachment still present after remove")
	}
}

func TestRemoveAttachment_NotFound(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if err := e.RemoveAttachment("ghost.txt"); err == nil {
		t.Error("expected not-found error")
	}
}
