package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// CombineCmd merges two entries within the same database. Non-conflicting
// fields are unioned; conflicting fields are resolved by --on-conflict. Tags
// are always unioned. Attachments with matching names are treated as field
// conflicts.
type CombineCmd struct {
	Src        string   `arg:"" help:"Source entry (provides fields to merge into Dst)."`
	Dst        string   `arg:"" help:"Destination entry (receives merged fields)."`
	Only       []string `help:"Restrict merge to these fields (title,username,password,url,notes,otp,tags,attachments,custom). Repeatable or comma-separated." placeholder:"FIELD"`
	OnConflict string   `default:"ask" enum:"ask,keep,overwrite,both" help:"On per-field conflict: ask, keep (dst wins), overwrite (src wins), both (preserve dst, save src as <field>.alt)."`
	DeleteSrc  bool     `name:"delete-src" help:"Delete src after a successful merge."`
	DryRun     bool     `name:"dry-run" help:"Show the merge plan without applying it."`
	Force      bool     `short:"f" help:"Skip the y/N confirmation prompt."`
}

// Help returns extended help for combine.
func (CombineCmd) Help() string {
	return `Merge two entries within the same database, e.g. attach an OTP-only
entry into an existing login.

Fields (title, username, password, url, notes, otp), custom fields, tags,
and attachments are compared between SRC and DST:

  • Empty dst fields adopt the src value (no conflict).
  • When both sides have a value, --on-conflict decides:
      ask       – interactive prompt for each conflict [1/2/b/s/a]
      keep      – keep dst, drop src
      overwrite – src wins, dst replaced
      both      – dst kept, src saved as <field>.alt

Tags are always unioned (src tags missing on dst are added).
Attachments with identical names and content are skipped silently.

Use --delete-src to remove the source entry after a successful merge.
Use --dry-run to preview the merge plan without applying it.`
}

func (cmd *CombineCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	src, err := c.db.ResolveEntry(cmd.Src)
	if err != nil {
		return err
	}
	dst, err := c.db.ResolveEntry(cmd.Dst)
	if err != nil {
		return err
	}
	if src.DisplayPath() == dst.DisplayPath() {
		return &UserError{Msg: "Source and destination are the same entry."}
	}

	only, err := parseOnlyFilter(cmd.Only)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	// One reader for the whole run — multiple bufio.NewReader calls would each
	// over-read from c.in and drop the buffered tail between prompts.
	reader := bufio.NewReader(c.in)

	plan, err := buildCombinePlan(src, dst, only, cmd.OnConflict, c, reader)
	if err != nil {
		return err
	}

	printCombinePlan(c.out, plan, src, dst, cmd.DeleteSrc)

	if cmd.DryRun {
		return nil
	}
	if !plan.hasChanges() && !cmd.DeleteSrc {
		fmt.Fprintln(c.out, color.Faint("Nothing to do."))
		return nil
	}

	// Safety: if --delete-src would silently throw away src fields the plan
	// resolved as Skip, refuse before any y/N. --force overrides.
	if cmd.DeleteSrc && !cmd.Force {
		if lost := lossySrcFields(src, only, plan); len(lost) > 0 {
			return &UserError{Msg: fmt.Sprintf(
				"--delete-src would discard src values: %s\nOptions:\n  - drop --delete-src to keep src as a separate entry\n  - --on-conflict=overwrite or both to migrate the values\n  - --force to delete anyway",
				strings.Join(lost, ", "))}
		}
	}

	if !cmd.Force {
		if c.gf.yes {
			fmt.Fprintf(c.out, "\n%s %s\n", color.Yellow("Apply"), color.Faint("(auto-yes)"))
		} else {
			fmt.Fprintf(c.out, "\n%s? [y/N]: ", color.Red("Apply"))
			line, _ := reader.ReadString('\n')
			reply := strings.ToLower(strings.TrimSpace(line))
			if reply != "y" && reply != "yes" {
				return &UserError{Msg: "Aborted."}
			}
		}
	}

	applyCombinePlan(dst, plan)
	if cmd.DeleteSrc {
		if err := c.db.DeleteEntry(src); err != nil {
			return &UserError{Msg: err.Error()}
		}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	fmt.Fprintf(c.out, "%s %s %s %s\n",
		color.Green("Combined"),
		color.Bold(src.DisplayPath()),
		color.Faint("→"),
		color.Bold(dst.DisplayPath()))
	if cmd.DeleteSrc {
		fmt.Fprintf(c.out, "%s %s\n",
			color.Yellow("Deleted"), color.Bold(src.DisplayPath()))
	}
	return nil
}

// --- plan model -------------------------------------------------------------

// combineAction is the resolved action for one field.
type combineAction string

const (
	actionSkip      combineAction = "skip"      // dst stays as is
	actionAdopt     combineAction = "adopt"     // dst is empty, take from src
	actionOverwrite combineAction = "overwrite" // src wins, dst replaced
	actionBoth      combineAction = "both"      // dst kept, src stashed as <field>.alt
)

type combineItem struct {
	Kind   string // "field", "custom", "attachment", "tag"
	Name   string // field/key/attachment name (empty for tags)
	SrcVal string // string repr of src value
	DstVal string // string repr of dst value
	Action combineAction
	Data   []byte // attachment payload from src, when relevant
}

type combinePlan struct {
	Items []combineItem
	Tags  []string // tags to ADD to dst (already filtered to those missing)
}

func (p *combinePlan) hasChanges() bool {
	if len(p.Tags) > 0 {
		return true
	}
	for _, it := range p.Items {
		if it.Action != actionSkip {
			return true
		}
	}
	return false
}

// --- plan building ----------------------------------------------------------

// standardFields enumerates the well-known fields, in display order.
var standardFields = []string{"title", "username", "password", "url", "notes", "otp"}

// parseOnlyFilter normalises --only=field,field repeats into a set. An empty
// list means "everything." Recognised tokens: standard fields above plus
// "tags", "attachments", "custom". Unknown tokens are an error.
func parseOnlyFilter(raw []string) (map[string]bool, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := map[string]bool{}
	for _, chunk := range raw {
		for _, t := range strings.Split(chunk, ",") {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			switch t {
			case "title", "username", "password", "url", "notes", "otp",
				"tags", "attachments", "custom":
				out[t] = true
			default:
				return nil, fmt.Errorf("unknown field in --only: %s", t)
			}
		}
	}
	return out, nil
}

func wantField(only map[string]bool, name string) bool {
	if only == nil {
		return true
	}
	return only[name]
}

// buildCombinePlan walks src + dst once and produces a per-field action list.
// Conflicts are resolved per `policy`; "ask" prompts the user via c.in/c.out.
func buildCombinePlan(src, dst *db.Entry, only map[string]bool, policy string, c *ctx, reader *bufio.Reader) (*combinePlan, error) {
	plan := &combinePlan{}

	// Standard fields.
	for _, f := range standardFields {
		if !wantField(only, f) {
			continue
		}
		sv, _ := src.GetAttribute(f)
		dv, _ := dst.GetAttribute(f)
		if sv == "" {
			continue
		}
		if dv == "" {
			plan.Items = append(plan.Items, combineItem{Kind: "field", Name: f, SrcVal: sv, DstVal: dv, Action: actionAdopt})
			continue
		}
		if sv == dv {
			continue
		}
		act, err := resolveConflict(c, reader, "field", f, sv, dv, policy)
		if err != nil {
			return nil, err
		}
		plan.Items = append(plan.Items, combineItem{Kind: "field", Name: f, SrcVal: sv, DstVal: dv, Action: act})
	}

	// Custom fields.
	if wantField(only, "custom") {
		srcCustom := src.CustomFields()
		dstCustom := dst.CustomFields()
		keys := make([]string, 0, len(srcCustom))
		for k := range srcCustom {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sv := srcCustom[k]
			dv := dstCustom[k]
			if sv == "" {
				continue
			}
			if dv == "" {
				plan.Items = append(plan.Items, combineItem{Kind: "custom", Name: k, SrcVal: sv, Action: actionAdopt})
				continue
			}
			if sv == dv {
				continue
			}
			act, err := resolveConflict(c, reader, "custom", k, sv, dv, policy)
			if err != nil {
				return nil, err
			}
			plan.Items = append(plan.Items, combineItem{Kind: "custom", Name: k, SrcVal: sv, DstVal: dv, Action: act})
		}
	}

	// Tags (always union; no conflict).
	if wantField(only, "tags") {
		dstSet := map[string]bool{}
		for _, t := range dst.Tags() {
			dstSet[strings.ToLower(t)] = true
		}
		for _, t := range src.Tags() {
			if dstSet[strings.ToLower(t)] {
				continue
			}
			plan.Tags = append(plan.Tags, t)
			dstSet[strings.ToLower(t)] = true
		}
	}

	// Attachments.
	if wantField(only, "attachments") {
		for _, name := range src.AttachmentList() {
			data, err := src.AttachmentContent(name)
			if err != nil {
				return nil, &UserError{Msg: err.Error()}
			}
			if !dst.AttachmentExists(name) {
				plan.Items = append(plan.Items, combineItem{Kind: "attachment", Name: name, SrcVal: fmt.Sprintf("%d bytes", len(data)), Action: actionAdopt, Data: data})
				continue
			}
			// Conflict: same-named attachment exists on dst. Compare bytes —
			// identical content means no real conflict, skip silently.
			dstData, err := dst.AttachmentContent(name)
			if err == nil && bytes.Equal(data, dstData) {
				continue
			}
			act, err := resolveConflict(c, reader, "attachment", name,
				fmt.Sprintf("%d bytes (src)", len(data)),
				fmt.Sprintf("%d bytes (dst)", len(dstData)),
				policy)
			if err != nil {
				return nil, err
			}
			plan.Items = append(plan.Items, combineItem{Kind: "attachment", Name: name, SrcVal: fmt.Sprintf("%d bytes", len(data)), DstVal: fmt.Sprintf("%d bytes", len(dstData)), Action: act, Data: data})
		}
	}

	return plan, nil
}

// resolveConflict maps the policy + (optionally) an interactive prompt into a
// concrete action for one field.
func resolveConflict(c *ctx, reader *bufio.Reader, kind, name, srcVal, dstVal, policy string) (combineAction, error) {
	switch policy {
	case "keep":
		return actionSkip, nil
	case "overwrite":
		return actionOverwrite, nil
	case "both":
		return actionBoth, nil
	}
	// "ask"
	if c.gf.yes {
		fmt.Fprintf(c.out, "  %s %s/%s %s\n", color.Faint("Conflict:"), kind, name, color.Faint("(auto-yes → keep dst)"))
		return actionSkip, nil
	}
	fmt.Fprintf(c.out, "\n%s %s\n", color.Yellow("Conflict:"), color.Bold(fmt.Sprintf("%s/%s", kind, name)))
	fmt.Fprintf(c.out, "  %s %s\n", color.Faint("[1] src:"), truncDisplay(srcVal))
	fmt.Fprintf(c.out, "  %s %s\n", color.Faint("[2] dst:"), truncDisplay(dstVal))
	fmt.Fprintf(c.out, "  %s\n", color.Faint("[b] keep both (src saved as "+name+".alt)"))
	fmt.Fprintf(c.out, "  %s\n", color.Faint("[s] skip (keep dst, drop src)"))
	fmt.Fprintf(c.out, "  %s\n", color.Faint("[a] abort"))
	fmt.Fprintf(c.out, "%s [1/2/b/s/a]: ", color.Bold("Choose"))
	line, _ := reader.ReadString('\n')
	reply := strings.ToLower(strings.TrimSpace(line))
	switch reply {
	case "1":
		return actionOverwrite, nil
	case "2", "":
		return actionSkip, nil
	case "b", "both":
		return actionBoth, nil
	case "s", "skip":
		return actionSkip, nil
	case "a", "abort":
		return "", &UserError{Msg: "Aborted."}
	default:
		return "", &UserError{Msg: fmt.Sprintf("Unknown choice: %q", reply)}
	}
}

// --- application ------------------------------------------------------------

func applyCombinePlan(dst *db.Entry, plan *combinePlan) {
	for _, it := range plan.Items {
		switch it.Kind {
		case "field":
			switch it.Action {
			case actionAdopt, actionOverwrite:
				dst.SetField(it.Name, it.SrcVal)
			case actionBoth:
				dst.SetField(it.Name+".alt", it.SrcVal)
			}
		case "custom":
			switch it.Action {
			case actionAdopt, actionOverwrite:
				dst.SetField(it.Name, it.SrcVal)
			case actionBoth:
				dst.SetField(it.Name+".alt", it.SrcVal)
			}
		case "attachment":
			switch it.Action {
			case actionAdopt:
				_ = dst.AddAttachment(it.Name, it.Data, false)
			case actionOverwrite:
				_ = dst.AddAttachment(it.Name, it.Data, true)
			case actionBoth:
				_ = dst.AddAttachment(altAttachmentName(it.Name), it.Data, true)
			}
		}
	}
	if len(plan.Tags) > 0 {
		merged := append([]string{}, dst.Tags()...)
		merged = append(merged, plan.Tags...)
		dst.SetTags(merged)
	}
}

// lossySrcFields lists src field names whose non-empty values would silently
// vanish if src is deleted: fields that the plan resolved as Skip (i.e.
// --on-conflict=keep, or "skip" picked at the ask prompt). Both-policy items
// are not lossy — the src value lands on dst as <field>.alt. Fields excluded
// by --only are treated as intentional (the user scoped the merge on purpose).
// Tags are never lossy (always unioned).
func lossySrcFields(_ *db.Entry, _ map[string]bool, plan *combinePlan) []string {
	var lost []string
	for _, it := range plan.Items {
		if it.Action != actionSkip {
			continue
		}
		switch it.Kind {
		case "field", "custom":
			lost = append(lost, it.Kind+"/"+it.Name)
		case "attachment":
			lost = append(lost, "attachment:"+it.Name)
		}
	}
	return lost
}

// altAttachmentName turns "foo.txt" into "foo.alt.txt"; "foo" into "foo.alt".
func altAttachmentName(name string) string {
	if i := strings.LastIndex(name, "."); i > 0 {
		return name[:i] + ".alt" + name[i:]
	}
	return name + ".alt"
}

// --- presentation -----------------------------------------------------------

func printCombinePlan(out interface{ Write(p []byte) (int, error) }, plan *combinePlan, src, dst *db.Entry, deleteSrc bool) {
	fmt.Fprintf(out, "%s %s %s %s\n",
		color.Bold("Combine plan:"),
		color.Bold(src.DisplayPath()),
		color.Faint("→"),
		color.Bold(dst.DisplayPath()))

	if !plan.hasChanges() {
		return
	}

	for _, it := range plan.Items {
		var verb, val string
		switch it.Action {
		case actionAdopt:
			verb = color.Green("+")
			val = it.SrcVal
		case actionOverwrite:
			verb = color.Yellow("~")
			val = fmt.Sprintf("%s %s %s",
				truncDisplay(it.DstVal), color.Faint("→"), truncDisplay(it.SrcVal))
		case actionBoth:
			verb = color.Cyan("b")
			val = fmt.Sprintf("dst kept; src → %s.alt", it.Name)
		case actionSkip:
			verb = color.Faint("=")
			val = "keep dst"
		}
		fmt.Fprintf(out, "  %s %s %s\n", verb, color.Bold(it.Kind+"/"+it.Name), val)
	}
	if len(plan.Tags) > 0 {
		fmt.Fprintf(out, "  %s %s %s\n",
			color.Green("+"), color.Bold("tags"),
			strings.Join(plan.Tags, ", "))
	}
	if deleteSrc {
		fmt.Fprintf(out, "  %s %s\n", color.Yellow("-"), color.Bold("delete src"))
	}
}

// truncDisplay shortens a value for the conflict prompt to a single line.
func truncDisplay(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}
