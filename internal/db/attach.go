package db

import (
	"fmt"
	"sort"
	"strings"
)

// AttachmentList returns the entry's attachment names sorted case-insensitively.
func (e *Entry) AttachmentList() []string {
	names := make([]string, 0, len(e.e.Binaries))
	for _, b := range e.e.Binaries {
		names = append(names, b.Name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

// AttachmentContent returns the bytes for the attachment with the given name.
func (e *Entry) AttachmentContent(name string) ([]byte, error) {
	idx := e.findBinary(name)
	if idx == -1 {
		return nil, fmt.Errorf("attachment not found: %s", name)
	}
	binary := e.d.Raw.FindBinary(e.e.Binaries[idx].Value.ID)
	if binary == nil {
		return nil, fmt.Errorf("attachment binary missing: %s", name)
	}
	return binary.GetContentBytes()
}

// AttachmentExists returns true if an attachment with the given name is on
// this entry.
func (e *Entry) AttachmentExists(name string) bool {
	return e.findBinary(name) != -1
}

// AddAttachment stores `data` as a new binary and references it from this
// entry under `name`. If force is true, an existing attachment with the same
// name is replaced (the old reference is removed and a fresh binary is added).
func (e *Entry) AddAttachment(name string, data []byte, force bool) error {
	if existing := e.findBinary(name); existing != -1 {
		if !force {
			return fmt.Errorf("attachment already exists: %s. Use --force to replace it", name)
		}
		e.e.Binaries = append(e.e.Binaries[:existing], e.e.Binaries[existing+1:]...)
	}
	bin := e.d.Raw.AddBinary(data)
	e.e.Binaries = append(e.e.Binaries, bin.CreateReference(name))
	return nil
}

// RemoveAttachment removes the named attachment from this entry.
func (e *Entry) RemoveAttachment(name string) error {
	idx := e.findBinary(name)
	if idx == -1 {
		return fmt.Errorf("attachment not found: %s", name)
	}
	e.e.Binaries = append(e.e.Binaries[:idx], e.e.Binaries[idx+1:]...)
	return nil
}

func (e *Entry) findBinary(name string) int {
	for i, b := range e.e.Binaries {
		if b.Name == name {
			return i
		}
	}
	return -1
}
