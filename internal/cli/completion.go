package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CompletionCmd generates shell completion scripts or provides dynamic
// completions via the hidden __complete sub-command.
type CompletionCmd struct {
	Shell string `arg:"" help:"Shell name: bash, zsh, or fish."`
}

func (cmd *CompletionCmd) Run(c *ctx) error {
	switch cmd.Shell {
	case "bash":
		fmt.Fprint(c.out, bashCompletionScript)
	case "zsh":
		fmt.Fprint(c.out, zshCompletionScript)
	case "fish":
		fmt.Fprint(c.out, fishCompletionScript)
	default:
		return &UserError{Msg: fmt.Sprintf("Unknown shell: %s. Use bash, zsh, or fish.", cmd.Shell)}
	}
	return nil
}

// CompleteCmd is the hidden helper that shell completion functions call to get
// dynamic completions (entry paths, profile names, etc.).
type CompleteCmd struct {
	Resource string `arg:"" help:"What to complete: commands, profiles, entries, attachments."`
	Profile  string `arg:"" optional:"" help:"Profile name for entry/attachment completion."`
	Entry    string `arg:"" optional:"" help:"Entry path for attachment completion."`
	Cur      string `arg:"" optional:"" help:"Current word (filter prefix)."`
}

func (cmd *CompleteCmd) Run(c *ctx) error {
	switch cmd.Resource {
	case "commands":
		for _, name := range allCommands {
			if cmd.Cur == "" || strings.HasPrefix(name, cmd.Cur) {
				fmt.Fprintln(c.out, name)
			}
		}
	case "profiles":
		for name := range c.fileConfig.Databases {
			if cmd.Cur == "" || strings.HasPrefix(name, cmd.Cur) {
				fmt.Fprintln(c.out, name)
			}
		}
	case "entries":
		if err := c.openDatabase(); err != nil {
			// Silently return — can't prompt for password during completion.
			return nil
		}
		for _, e := range c.db.SortedEntries() {
			p := e.DisplayPath()
			if cmd.Cur == "" || strings.HasPrefix(p, cmd.Cur) {
				fmt.Fprintln(c.out, p)
			}
		}
	case "tags":
		if err := c.openDatabase(); err != nil {
			return nil
		}
		for _, t := range sortedUniqueTags(c.db) {
			if cmd.Cur == "" || strings.HasPrefix(strings.ToLower(t), strings.ToLower(cmd.Cur)) {
				fmt.Fprintln(c.out, t)
			}
		}
	case "attachments":
		if err := c.openDatabase(); err != nil {
			return nil
		}
		if cmd.Entry == "" {
			return nil
		}
		entry, err := c.db.ResolveEntry(cmd.Entry)
		if err != nil {
			return nil
		}
		for _, name := range entry.AttachmentList() {
			if cmd.Cur == "" || strings.HasPrefix(name, cmd.Cur) {
				fmt.Fprintln(c.out, name)
			}
		}
	default:
		return &UserError{Msg: fmt.Sprintf("Unknown completion resource: %s", cmd.Resource)}
	}
	return nil
}

// allCommands is the canonical list of top-level command names (no aliases).
var allCommands = []string{
	"ls", "search",
	"get",
	"copy",
	"pick",
	"insert",
	"edit",
	"generate",
	"remove",
	"move",
	"duplicate",
	"mkdir",
	"merge",
	"audit",
	"stats",
	"open",
	"clean",
	"export",
	"import",
	"import-pass",
	"tags",
	"tag",
	"combine",
	"doctor",
	"db",
	"attach",
	"completion",
}

// --- cache helpers for offline completion ---

// CompletionCacheDir returns the directory where per-profile entry path caches are stored.
func CompletionCacheDir() string {
	cacheRoot := os.Getenv("XDG_CACHE_HOME")
	if cacheRoot == "" {
		home, _ := os.UserHomeDir()
		cacheRoot = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheRoot, "kpass", "completions")
}

// WriteEntryCache writes entry paths for the given profile to the cache file.
func WriteEntryCache(profile string, paths []string) error {
	dir := CompletionCacheDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Sanitize profile name for filename.
	safe := strings.Map(func(r rune) rune {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, profile)
	f, err := os.Create(filepath.Join(dir, safe+".paths"))
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range paths {
		fmt.Fprintln(f, p)
	}
	return f.Close()
}

// --- static completion scripts ---

// bashCompletionScript is the bash completion function. It delegates dynamic
// completions to `kpass __complete`.
const bashCompletionScript = `# kpass bash completion — source this file or place it in
# /etc/bash_completion.d/ or ~/.local/share/bash-completion/completions/
_kpass() {
    local cur prev words cword
    _init_completion || return

    local kpass_cmd="${words[0]}"

    # --- @profile completion (after the command, before args) ---
    if [[ "$cur" == @* ]]; then
        local profiles
        profiles=$("$kpass_cmd" __complete profiles "" "" "${cur#@}" 2>/dev/null)
        if [[ -n "$profiles" ]]; then
            COMPREPLY=($(compgen -W "$profiles" -P "@" -- "${cur#@}"))
            return
        fi
    fi

    # --- command completion (first positional) ---
    if [[ $cword -eq 1 ]]; then
        local cmds
        cmds=$("$kpass_cmd" __complete commands "" "" "$cur" 2>/dev/null)
        COMPREPLY=($(compgen -W "$cmds" -- "$cur"))
        return
    fi

    local cmd="${words[1]}"
    # Map aliases to canonical names for flag detection.
    case "$cmd" in
        s) cmd="search" ;;
        g) cmd="get" ;;
        c) cmd="copy" ;;
        i) cmd="insert" ;;
        e) cmd="edit" ;;
        rm) cmd="remove" ;;
        mv) cmd="move" ;;
        dup) cmd="duplicate" ;;
    esac

    prev="${words[$((cword-1))]}"

    # --- flag completion ---
    case "$prev" in
        --config|--database|--password-file|--key-file)
            _filedir
            return ;;
        --field|-F)
            local fields="path title username password url notes otp"
            COMPREPLY=($(compgen -W "$fields" -- "$cur"))
            return ;;
        --tag|--tag-any)
            local tags
            tags=$("$kpass_cmd" __complete tags "" "" "$cur" 2>/dev/null)
            COMPREPLY=($(compgen -W "$tags" -- "$cur"))
            return ;;
    esac

    if [[ "$cur" == -* ]]; then
        # Global flags + per-command flags.
        local global_flags="--config --database --password-file --key-file --cache-ttl --session-ttl --no-cache --no-session --no-color --help --version"
        case "$cmd" in
            ls)       COMPREPLY=($(compgen -W "$global_flags --flat --groups --long -l --depth --tag --tag-any --json" -- "$cur")) ;;
            search|s) COMPREPLY=($(compgen -W "$global_flags --field -F --flat --ignore-case -i --tag --tag-any --json" -- "$cur")) ;;
            get|g)    COMPREPLY=($(compgen -W "$global_flags --field -F" -- "$cur")) ;;
            copy|c)   COMPREPLY=($(compgen -W "$global_flags --field -F" -- "$cur")) ;;
            pick)     COMPREPLY=($(compgen -W "$global_flags --action --field -F --timeout --preview --tag --tag-any" -- "$cur")) ;;
            tags)     COMPREPLY=($(compgen -W "$global_flags --sort --names --json" -- "$cur")) ;;
            combine)  COMPREPLY=($(compgen -W "$global_flags --only --on-conflict --delete-src --dry-run --force -f" -- "$cur")) ;;
            tag)
                local sub="${words[2]}"
                case "$sub" in
                    add|remove|rm|rename|mv) COMPREPLY=($(compgen -W "$global_flags" -- "$cur")) ;;
                    *) COMPREPLY=($(compgen -W "add remove rename" -- "$cur")) ;;
                esac
                return ;;
            insert|i) COMPREPLY=($(compgen -W "$global_flags --username -u --url --notes --otp --force -f --password --password-stdin --generate -g --length -L --lower --upper --digits --symbols" -- "$cur")) ;;
            edit|e)   COMPREPLY=($(compgen -W "$global_flags --editor --rename --username -u --url --notes --otp --clear --password --password-stdin --generate -g --length -L --lower --upper --digits --symbols" -- "$cur")) ;;
            generate) COMPREPLY=($(compgen -W "$global_flags --timeout --username -u --url --notes --otp --force -f --length -L --lower --upper --digits --symbols --copy --clip" -- "$cur")) ;;
            remove|rm) COMPREPLY=($(compgen -W "$global_flags --force -f" -- "$cur")) ;;
            move|mv)  COMPREPLY=($(compgen -W "$global_flags --force -f" -- "$cur")) ;;
            duplicate|dup) COMPREPLY=($(compgen -W "$global_flags --force -f" -- "$cur")) ;;
            attach)
                local sub="${words[2]}"
                case "$sub" in
                    ls)      COMPREPLY=($(compgen -W "" -- "$cur")) ;;
                    add)     COMPREPLY=($(compgen -W "$global_flags --name --force -f" -- "$cur")) ;;
                    remove)  COMPREPLY=($(compgen -W "" -- "$cur")) ;;
                    extract) COMPREPLY=($(compgen -W "$global_flags --force -f" -- "$cur")) ;;
                    *)       COMPREPLY=($(compgen -W "ls add remove extract" -- "$cur")) ;;
                esac
                return ;;
            merge)    COMPREPLY=($(compgen -W "$global_flags --source-password-file --source-key-file --on-conflict --rename-suffix" -- "$cur")) ;;
            db)
                local sub="${words[2]}"
                case "$sub" in
                    add)     COMPREPLY=($(compgen -W "$global_flags --password-database --password-entry --default" -- "$cur")) ;;
                    *)       COMPREPLY=($(compgen -W "ls add rm default" -- "$cur")) ;;
                esac
                return ;;
            completion) COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur")); return ;;
            doctor|mkdir) COMPREPLY=($(compgen -W "$global_flags" -- "$cur")) ;;
            *)        COMPREPLY=($(compgen -W "$global_flags" -- "$cur")) ;;
        esac
        return
    fi

    # --- positional argument completion (entry paths, etc.) ---
    case "$cmd" in
        ls)
            # Optional: group path (same as entry path prefix).
            local entries
            entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
            COMPREPLY=($(compgen -W "$entries" -- "$cur"))
            ;;
        combine)
            local entries
            entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
            COMPREPLY=($(compgen -W "$entries" -- "$cur"))
            ;;
        search|s|get|g|copy|c|edit|e|remove|rm|generate)
            local entries
            entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
            COMPREPLY=($(compgen -W "$entries" -- "$cur"))
            ;;
        pick)
            # Optional query — free-text, no completion.
            ;;
        tag)
            local sub="${words[2]}"
            if [[ -z "$sub" || "$sub" == "$cur" ]]; then
                COMPREPLY=($(compgen -W "add remove rm rename mv" -- "$cur"))
                return
            fi
            # Position within the subcommand args (ignoring flags).
            local arg_count=0
            for w in "${words[@]:3}"; do
                [[ "$w" == -* ]] && continue
                ((arg_count++))
            done
            case "$sub" in
                add|remove|rm)
                    if [[ $arg_count -le 1 ]]; then
                        local tags
                        tags=$("$kpass_cmd" __complete tags "" "" "$cur" 2>/dev/null)
                        COMPREPLY=($(compgen -W "$tags" -- "$cur"))
                    else
                        local entries
                        entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
                        COMPREPLY=($(compgen -W "$entries" -- "$cur"))
                    fi
                    ;;
                rename|mv)
                    if [[ $arg_count -le 1 ]]; then
                        local tags
                        tags=$("$kpass_cmd" __complete tags "" "" "$cur" 2>/dev/null)
                        COMPREPLY=($(compgen -W "$tags" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        insert|i|mkdir|move|mv|duplicate|dup)
            # Partial paths: allow free-form but suggest existing paths.
            local entries
            entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
            COMPREPLY=($(compgen -W "$entries" -- "$cur"))
            ;;
        merge)
            _filedir kdbx
            ;;
        attach)
            local sub="${words[2]}"
            if [[ "$sub" == "add" ]]; then
                local arg_count=0
                for w in "${words[@]:2}"; do
                    [[ "$w" == -* ]] && continue
                    ((arg_count++))
                done
                if [[ $arg_count -eq 1 ]]; then
                    local entries
                    entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
                    COMPREPLY=($(compgen -W "$entries" -- "$cur"))
                elif [[ $arg_count -eq 2 ]]; then
                    _filedir
                fi
            elif [[ "$sub" == "remove" || "$sub" == "extract" || "$sub" == "ls" ]]; then
                local arg_count=0
                for w in "${words[@]:2}"; do
                    [[ "$w" == -* ]] && continue
                    ((arg_count++))
                done
                if [[ $arg_count -eq 1 ]]; then
                    local entries
                    entries=$("$kpass_cmd" __complete entries "" "" "$cur" 2>/dev/null)
                    COMPREPLY=($(compgen -W "$entries" -- "$cur"))
                elif [[ $arg_count -eq 2 ]]; then
                    # Complete attachment name — needs database access.
                    local entry="${words[3]}"
                    local atts
                    atts=$("$kpass_cmd" __complete attachments "" "$entry" "$cur" 2>/dev/null)
                    COMPREPLY=($(compgen -W "$atts" -- "$cur"))
                fi
            fi
            ;;
    esac
}
complete -F _kpass kpass
`

// zshCompletionScript is the zsh completion function.
const zshCompletionScript = `#compdef kpass
# kpass zsh completion — place in a directory in your $fpath.

_kpass_commands() {
    local -a cmds
    cmds=(${(f)"$($words[1] __complete commands 2>/dev/null)"})
    _describe 'command' cmds
}

_kpass_profiles() {
    local -a profiles
    profiles=(${(f)"$($words[1] __complete profiles 2>/dev/null)"})
    if [[ -n "$profiles" ]]; then
        _describe 'profile' profiles -P @
    fi
}

_kpass_entries() {
    local -a entries
    entries=(${(f)"$($words[1] __complete entries '' '' "$words[CURRENT]" 2>/dev/null)"})
    if [[ -n "$entries" ]]; then
        _describe 'entry' entries
    fi
}

_kpass_fields() {
    local -a fields
    fields=('path' 'title' 'username' 'password' 'url' 'notes' 'otp')
    _describe 'field' fields
}

_kpass_tags() {
    local -a tags
    tags=(${(f)"$($words[1] __complete tags '' '' "$words[CURRENT]" 2>/dev/null)"})
    if [[ -n "$tags" ]]; then
        _describe 'tag' tags
    fi
}

_kpass_tag_actions() {
    local -a actions
    actions=('add:Add a tag to entries' 'remove:Remove a tag from entries' 'rename:Rename a tag everywhere')
    _describe 'action' actions
}

_kpass_attach_actions() {
    local -a actions
    actions=('ls' 'add' 'remove' 'extract')
    _describe 'action' actions
}

_kpass_db_actions() {
    local -a actions
    actions=('ls' 'add' 'rm' 'default')
    _describe 'action' actions
}

_kpass() {
    local context state state_descr line
    typeset -A opt_args

    _arguments -C \
        '--config[Path to config file]:file:_files' \
        '--database[Override database path]:file:_files' \
        '--password-file[Read master password from file]:file:_files' \
        '--key-file[Composite key file]:file:_files' \
        '--cache-ttl[Cache TTL in seconds]: :' \
        '--session-ttl[Cache TTL in seconds]: :' \
        '--no-cache[Disable password cache]' \
        '--no-session[Disable password cache]' \
        '--no-color[Disable colored output]' \
        '(-V --version)'{-V,--version}'[Print version]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '1: :_kpass_commands' \
        '*:: :->args'

    case "$state" in
        args)
            local cmd="${words[2]}"
            case "$cmd" in
                s) cmd="search" ;;
                g) cmd="get" ;;
                c) cmd="copy" ;;
                i) cmd="insert" ;;
                e) cmd="edit" ;;
                rm) cmd="remove" ;;
                mv) cmd="move" ;;
                dup) cmd="duplicate" ;;
            esac
            case "$cmd" in
                ls)
                    _arguments \
                        '--flat[Print plain paths]' \
                        '--groups[List groups only]' \
                        '(-l --long)'{-l,--long}'[Table format]' \
                        '--depth[Limit tree depth]: :' \
                        '*--tag[Filter by tag (AND)]:tag:_kpass_tags' \
                        '*--tag-any[Filter by tag (OR)]:tag:_kpass_tags' \
                        '--json[Output as JSON]' \
                        '*:group or entry:_kpass_entries'
                    ;;
                search)
                    _arguments \
                        '*-F[Field to search]:field:_kpass_fields' \
                        '--flat[Print plain paths]' \
                        '(-i --ignore-case)'{-i,--ignore-case}'[Case-insensitive]' \
                        '*--tag[Filter by tag (AND)]:tag:_kpass_tags' \
                        '*--tag-any[Filter by tag (OR)]:tag:_kpass_tags' \
                        '--json[Output as JSON]' \
                        '1:term:'
                    ;;
                get)
                    _arguments \
                        '*-F[Field to print]:field:_kpass_fields' \
                        '1:entry:_kpass_entries'
                    ;;
                copy)
                    _arguments \
                        '*-F[Field to copy]:field:_kpass_fields' \
                        '1:entry:_kpass_entries' \
                        '2:timeout:'
                    ;;
                pick)
                    _arguments \
                        '--action[Action]:action:(get copy edit open show delete otp)' \
                        '-F[Field]:field:_kpass_fields' \
                        '--timeout[Clipboard timeout]: :' \
                        '--preview[Show fzf preview pane]' \
                        '*--tag[Filter by tag (AND)]:tag:_kpass_tags' \
                        '*--tag-any[Filter by tag (OR)]:tag:_kpass_tags'
                    ;;
                tags)
                    _arguments \
                        '--sort[Sort order]:order:(count name)' \
                        '--names[Print just tag names]' \
                        '--json[Output as JSON]'
                    ;;
                combine)
                    _arguments \
                        '*--only[Restrict to fields]:field:(title username password url notes otp tags attachments custom)' \
                        '--on-conflict[Conflict policy]:policy:(ask keep overwrite both)' \
                        '--delete-src[Delete src after merge]' \
                        '--dry-run[Show plan, do not apply]' \
                        '(-f --force)'{-f,--force}'[Skip y/N prompt]' \
                        '1:src:_kpass_entries' \
                        '2:dst:_kpass_entries'
                    ;;
                tag)
                    _arguments '1:action:_kpass_tag_actions' '*:: :->tag_args'
                    ;;
                insert)
                    _arguments \
                        '(-u --username)'{-u,--username}'[Username]: :' \
                        '--url[URL]: :' \
                        '--notes[Notes]: :' \
                        '--otp[OTP URI]: :' \
                        '(-f --force)'{-f,--force}'[Replace existing]' \
                        '--password[Password inline]: :' \
                        '--password-stdin[Read password from stdin]' \
                        '(-g --generate)'{-g,--generate}'[Generate password]' \
                        '(-L --length)'{-L,--length}'[Password length]: :' \
                        '--lower[Allow lowercase]' \
                        '--upper[Allow uppercase]' \
                        '--digits[Allow digits]' \
                        '--symbols[Allow symbols]' \
                        '1:path:'
                    ;;
                edit)
                    _arguments \
                        '--editor[Editor command]: :' \
                        '--rename[New title]: :' \
                        '(-u --username)'{-u,--username}'[Username]: :' \
                        '--url[URL]: :' \
                        '--notes[Notes]: :' \
                        '--otp[OTP URI]: :' \
                        '--clear[Clear field]:field:(username password url notes otp)' \
                        '--password[Password inline]: :' \
                        '--password-stdin[Read password from stdin]' \
                        '(-g --generate)'{-g,--generate}'[Generate password]' \
                        '(-L --length)'{-L,--length}'[Password length]: :' \
                        '1:entry:_kpass_entries'
                    ;;
                remove)
                    _arguments \
                        '(-f --force)'{-f,--force}'[Skip confirmation]' \
                        '1:entry:_kpass_entries'
                    ;;
                move)
                    _arguments \
                        '(-f --force)'{-f,--force}'[Overwrite destination]' \
                        '1:source:_kpass_entries' \
                        '2:destination:'
                    ;;
                duplicate)
                    _arguments \
                        '(-f --force)'{-f,--force}'[Overwrite destination]' \
                        '1:source:_kpass_entries' \
                        '2:destination:'
                    ;;
                generate)
                    _arguments \
                        '--timeout[Clipboard timeout]: :' \
                        '(-u --username)'{-u,--username}'[Username]: :' \
                        '--url[URL]: :' \
                        '--notes[Notes]: :' \
                        '--otp[OTP URI]: :' \
                        '(-f --force)'{-f,--force}'[Replace existing]' \
                        '(-L --length)'{-L,--length}'[Password length]: :' \
                        '--lower[Allow lowercase]' \
                        '--upper[Allow uppercase]' \
                        '--digits[Allow digits]' \
                        '--symbols[Allow symbols]' \
                        '--copy[Copy to clipboard]' \
                        '1:entry:_kpass_entries'
                    ;;
                attach)
                    _arguments '1:action:_kpass_attach_actions' '*:: :->attach_args'
                    ;;
                merge)
                    _arguments \
                        '--source-password-file[Source password file]:file:_files' \
                        '--source-key-file[Source key file]:file:_files' \
                        '--on-conflict[Conflict strategy]:(error skip overwrite rename)' \
                        '--rename-suffix[Rename suffix]: :' \
                        '1:source db:_files -g "*.kdbx"'
                    ;;
                db)
                    _arguments '1:action:_kpass_db_actions' '*:: :->db_args'
                    ;;
                completion)
                    _arguments '1:shell:(bash zsh fish)'
                    ;;
                doctor|mkdir)
                    ;;
            esac
            ;;
    esac

    case "$state" in
        tag_args)
            local sub="${words[2]}"
            case "$sub" in
                add|remove|rm)
                    _arguments '1:tag:_kpass_tags' '*:entry:_kpass_entries'
                    ;;
                rename|mv)
                    _arguments '1:old tag:_kpass_tags' '2:new tag:'
                    ;;
            esac
            ;;
        attach_args)
            local sub="${words[2]}"
            case "$sub" in
                ls) _arguments '1:entry:_kpass_entries' ;;
                add) _arguments '1:entry:_kpass_entries' '2:file:_files' ;;
                remove) _arguments '1:entry:_kpass_entries' '2:attachment:' ;;
                extract) _arguments '1:entry:_kpass_entries' '2:attachment:' '3:output:_files' ;;
            esac
            ;;
        db_args)
            local sub="${words[2]}"
            case "$sub" in
                add) _arguments '1:profile name:' '2:database file:_files' ;;
                rm) _arguments '1:profile:_kpass_profiles' ;;
                default) _arguments '1:profile:_kpass_profiles' ;;
            esac
            ;;
    esac
}

_kpass "$@"
`

// fishCompletionScript is the fish completion script.
const fishCompletionScript = `# kpass fish completion — place in ~/.config/fish/completions/kpass.fish

# Global flags
complete -c kpass -l config -d "Path to config file" -r -F
complete -c kpass -l database -d "Override database path" -r -F
complete -c kpass -l password-file -d "Read master password from file" -r -F
complete -c kpass -l key-file -d "Composite key file" -r -F
complete -c kpass -l cache-ttl -d "Cache TTL in seconds" -r
complete -c kpass -l session-ttl -d "Cache TTL in seconds" -r
complete -c kpass -l no-cache -d "Disable password cache"
complete -c kpass -l no-session -d "Disable password cache"
complete -c kpass -l no-color -d "Disable colored output"
complete -c kpass -s V -l version -d "Print version"
complete -c kpass -s h -l help -d "Show help"

# Subcommands
complete -c kpass -n "__fish_use_subcommand" -a ls -d "List entries as a tree"
complete -c kpass -n "__fish_use_subcommand" -a search -d "Find entries by name, path, or fields"
complete -c kpass -n "__fish_use_subcommand" -a get -d "Show one entry or one field"
complete -c kpass -n "__fish_use_subcommand" -a copy -d "Copy a field to the clipboard"
complete -c kpass -n "__fish_use_subcommand" -a pick -d "Pick an entry interactively"
complete -c kpass -n "__fish_use_subcommand" -a insert -d "Create a new entry"
complete -c kpass -n "__fish_use_subcommand" -a edit -d "Edit an existing entry"
complete -c kpass -n "__fish_use_subcommand" -a generate -d "Generate and store a password"
complete -c kpass -n "__fish_use_subcommand" -a remove -d "Delete an entry"
complete -c kpass -n "__fish_use_subcommand" -a move -d "Move or rename an entry"
complete -c kpass -n "__fish_use_subcommand" -a duplicate -d "Duplicate an entry"
complete -c kpass -n "__fish_use_subcommand" -a mkdir -d "Create a group path"
complete -c kpass -n "__fish_use_subcommand" -a merge -d "Import entries from another database"
complete -c kpass -n "__fish_use_subcommand" -a doctor -d "Validate config and profiles"
complete -c kpass -n "__fish_use_subcommand" -a db -d "Manage database profiles"
complete -c kpass -n "__fish_use_subcommand" -a attach -d "Manage entry attachments"
complete -c kpass -n "__fish_use_subcommand" -a tags -d "List tags with entry counts"
complete -c kpass -n "__fish_use_subcommand" -a tag -d "Bulk tag operations"
complete -c kpass -n "__fish_use_subcommand" -a import-pass -d "Import from pass(1)"
complete -c kpass -n "__fish_use_subcommand" -a open -d "Open entry URL in browser"
complete -c kpass -n "__fish_use_subcommand" -a audit -d "Check database for security issues"
complete -c kpass -n "__fish_use_subcommand" -a clean -d "Remove empty groups"
complete -c kpass -n "__fish_use_subcommand" -a export -d "Export entries to JSON/CSV"
complete -c kpass -n "__fish_use_subcommand" -a import -d "Import entries from JSON/CSV"
complete -c kpass -n "__fish_use_subcommand" -a history -d "View/restore entry history"
complete -c kpass -n "__fish_use_subcommand" -a undo -d "Restore from backup"
complete -c kpass -n "__fish_use_subcommand" -a completion -d "Generate shell completion"

# Dynamic tag completion (uses __complete)
function __kpass_tags
    set -l cmd (commandline -opc)
    $cmd[1] __complete tags "" "" "" 2>/dev/null
end

# ls flags
complete -c kpass -n "__fish_seen_subcommand_from ls" -l flat -d "Print plain paths"
complete -c kpass -n "__fish_seen_subcommand_from ls" -l groups -d "List groups only"
complete -c kpass -n "__fish_seen_subcommand_from ls" -l long -s l -d "Table format"
complete -c kpass -n "__fish_seen_subcommand_from ls" -l depth -d "Limit tree depth" -r
complete -c kpass -n "__fish_seen_subcommand_from ls" -l tag -d "Filter by tag (AND)" -r -a "(__kpass_tags)"
complete -c kpass -n "__fish_seen_subcommand_from ls" -l tag-any -d "Filter by tag (OR)" -r -a "(__kpass_tags)"
complete -c kpass -n "__fish_seen_subcommand_from ls" -l json -d "Output as JSON"

# search flags
complete -c kpass -n "__fish_seen_subcommand_from search s" -s F -l field -d "Field to search" -r -a "path title username password url notes otp"
complete -c kpass -n "__fish_seen_subcommand_from search s" -l flat -d "Print plain paths"
complete -c kpass -n "__fish_seen_subcommand_from search s" -s i -l ignore-case -d "Case-insensitive match"
complete -c kpass -n "__fish_seen_subcommand_from search s" -l tag -d "Filter by tag (AND)" -r -a "(__kpass_tags)"
complete -c kpass -n "__fish_seen_subcommand_from search s" -l tag-any -d "Filter by tag (OR)" -r -a "(__kpass_tags)"
complete -c kpass -n "__fish_seen_subcommand_from search s" -l json -d "Output as JSON"

# get flags
complete -c kpass -n "__fish_seen_subcommand_from get g" -s F -l field -d "Field to print" -r -a "path title username password url notes otp"

# copy flags
complete -c kpass -n "__fish_seen_subcommand_from copy c" -s F -l field -d "Field to copy" -r -a "path title username password url notes otp"

# pick flags
complete -c kpass -n "__fish_seen_subcommand_from pick" -l action -d "What to do" -r -a "get copy edit open show delete otp"
complete -c kpass -n "__fish_seen_subcommand_from pick" -s F -l field -d "Field to pass to action" -r -a "path title username password url notes otp"
complete -c kpass -n "__fish_seen_subcommand_from pick" -l timeout -d "Clipboard timeout" -r
complete -c kpass -n "__fish_seen_subcommand_from pick" -l preview -d "Show fzf preview"
complete -c kpass -n "__fish_seen_subcommand_from pick" -l tag -d "Filter by tag (AND)" -r -a "(__kpass_tags)"
complete -c kpass -n "__fish_seen_subcommand_from pick" -l tag-any -d "Filter by tag (OR)" -r -a "(__kpass_tags)"

# tags flags
complete -c kpass -n "__fish_seen_subcommand_from tags" -l sort -d "Sort order" -r -a "count name"
complete -c kpass -n "__fish_seen_subcommand_from tags" -l names -d "Print just tag names"
complete -c kpass -n "__fish_seen_subcommand_from tags" -l json -d "Output as JSON"

# tag subcommands
complete -c kpass -n "__fish_seen_subcommand_from tag; and not __fish_seen_subcommand_from add remove rm rename mv" -a add -d "Add a tag to entries"
complete -c kpass -n "__fish_seen_subcommand_from tag; and not __fish_seen_subcommand_from add remove rm rename mv" -a remove -d "Remove a tag from entries"
complete -c kpass -n "__fish_seen_subcommand_from tag; and not __fish_seen_subcommand_from add remove rm rename mv" -a rename -d "Rename a tag"

# import-pass flags
complete -c kpass -n "__fish_seen_subcommand_from import-pass" -l gpg-binary -d "GPG binary" -r
complete -c kpass -n "__fish_seen_subcommand_from import-pass" -l on-conflict -d "Conflict strategy" -r -a "error skip overwrite rename"
complete -c kpass -n "__fish_seen_subcommand_from import-pass" -s f -l force -d "Skip prompt"

# combine flags
complete -c kpass -n "__fish_seen_subcommand_from combine" -l only -d "Restrict to fields" -r -a "title username password url notes otp tags attachments custom"
complete -c kpass -n "__fish_seen_subcommand_from combine" -l on-conflict -d "Conflict policy" -r -a "ask keep overwrite both"
complete -c kpass -n "__fish_seen_subcommand_from combine" -l delete-src -d "Delete src after merge"
complete -c kpass -n "__fish_seen_subcommand_from combine" -l dry-run -d "Show plan only"
complete -c kpass -n "__fish_seen_subcommand_from combine" -s f -l force -d "Skip y/N prompt"

# combine positionals (use the same dynamic entry completion as other commands)
complete -c kpass -n "__fish_seen_subcommand_from combine" -a "(__kpass_entries (commandline -ct))"

# history flags
complete -c kpass -n "__fish_seen_subcommand_from history" -l diff -d "Show diff vs current"
complete -c kpass -n "__fish_seen_subcommand_from history" -l restore -d "Restore version by index" -r

# undo flags
complete -c kpass -n "__fish_seen_subcommand_from undo" -s l -l list -d "List backups"
complete -c kpass -n "__fish_seen_subcommand_from undo" -s f -l force -d "Skip confirmation"
complete -c kpass -n "__fish_seen_subcommand_from undo" -l index -d "Backup index" -r

# open flags
complete -c kpass -n "__fish_seen_subcommand_from open" -s F -l field -d "Field to open" -r -a "url otp"

# audit flags
complete -c kpass -n "__fish_seen_subcommand_from audit" -l json -d "Output as JSON"

# clean flags
complete -c kpass -n "__fish_seen_subcommand_from clean" -s f -l force -d "Skip confirmation"
complete -c kpass -n "__fish_seen_subcommand_from clean" -l json -d "Output as JSON"

# export flags
complete -c kpass -n "__fish_seen_subcommand_from export" -s o -l format -d "Output format" -r -a "json csv"
complete -c kpass -n "__fish_seen_subcommand_from export" -l output -d "Write to file" -r -F
complete -c kpass -n "__fish_seen_subcommand_from export" -s f -l force -d "Overwrite existing file"

# import flags
complete -c kpass -n "__fish_seen_subcommand_from import" -s o -l format -d "Input format" -r -a "json csv"
complete -c kpass -n "__fish_seen_subcommand_from import" -l on-conflict -d "Conflict strategy" -r -a "error skip overwrite rename"
complete -c kpass -n "__fish_seen_subcommand_from import" -s f -l force -d "Skip prompt"

# insert flags
complete -c kpass -n "__fish_seen_subcommand_from insert i" -s u -l username -d "Username" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l url -d "URL" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l notes -d "Notes" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l otp -d "OTP URI" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -s f -l force -d "Replace existing entry"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l password -d "Password inline" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l password-stdin -d "Read password from stdin"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -s g -l generate -d "Generate password"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -s L -l length -d "Password length" -r
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l lower -d "Allow lowercase"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l upper -d "Allow uppercase"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l digits -d "Allow digits"
complete -c kpass -n "__fish_seen_subcommand_from insert i" -l symbols -d "Allow symbols"

# edit flags
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l editor -d "Editor command" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l rename -d "New title" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -s u -l username -d "Username" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l url -d "URL" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l notes -d "Notes" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l otp -d "OTP URI" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l clear -d "Clear a field" -r -a "username password url notes otp"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l password -d "Password inline" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l password-stdin -d "Read password from stdin"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -s g -l generate -d "Generate password"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -s L -l length -d "Password length" -r
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l lower -d "Allow lowercase"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l upper -d "Allow uppercase"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l digits -d "Allow digits"
complete -c kpass -n "__fish_seen_subcommand_from edit e" -l symbols -d "Allow symbols"

# remove flags
complete -c kpass -n "__fish_seen_subcommand_from remove rm" -s f -l force -d "Skip confirmation"

# move flags
complete -c kpass -n "__fish_seen_subcommand_from move mv" -s f -l force -d "Overwrite destination"

# duplicate flags
complete -c kpass -n "__fish_seen_subcommand_from duplicate dup" -s f -l force -d "Overwrite destination"

# generate flags
complete -c kpass -n "__fish_seen_subcommand_from generate" -l timeout -d "Clipboard timeout" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -s u -l username -d "Username" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -l url -d "URL" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -l notes -d "Notes" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -l otp -d "OTP URI" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -s f -l force -d "Replace password"
complete -c kpass -n "__fish_seen_subcommand_from generate" -s L -l length -d "Password length" -r
complete -c kpass -n "__fish_seen_subcommand_from generate" -l lower -d "Allow lowercase"
complete -c kpass -n "__fish_seen_subcommand_from generate" -l upper -d "Allow uppercase"
complete -c kpass -n "__fish_seen_subcommand_from generate" -l digits -d "Allow digits"
complete -c kpass -n "__fish_seen_subcommand_from generate" -l symbols -d "Allow symbols"
complete -c kpass -n "__fish_seen_subcommand_from generate" -l copy -d "Copy to clipboard"

# merge flags
complete -c kpass -n "__fish_seen_subcommand_from merge" -l source-password-file -d "Source password file" -r -F
complete -c kpass -n "__fish_seen_subcommand_from merge" -l source-key-file -d "Source key file" -r -F
complete -c kpass -n "__fish_seen_subcommand_from merge" -l on-conflict -d "Conflict strategy" -r -a "error skip overwrite rename"
complete -c kpass -n "__fish_seen_subcommand_from merge" -l rename-suffix -d "Rename suffix" -r

# db subcommands
complete -c kpass -n "__fish_seen_subcommand_from db; and not __fish_seen_subcommand_from ls add rm default" -a ls -d "List profiles"
complete -c kpass -n "__fish_seen_subcommand_from db; and not __fish_seen_subcommand_from ls add rm default" -a add -d "Add a profile"
complete -c kpass -n "__fish_seen_subcommand_from db; and not __fish_seen_subcommand_from ls add rm default" -a rm -d "Remove a profile"
complete -c kpass -n "__fish_seen_subcommand_from db; and not __fish_seen_subcommand_from ls add rm default" -a default -d "Show/change default"

# attach subcommands
complete -c kpass -n "__fish_seen_subcommand_from attach; and not __fish_seen_subcommand_from ls add remove extract" -a ls -d "List attachments"
complete -c kpass -n "__fish_seen_subcommand_from attach; and not __fish_seen_subcommand_from ls add remove extract" -a add -d "Add attachment"
complete -c kpass -n "__fish_seen_subcommand_from attach; and not __fish_seen_subcommand_from ls add remove extract" -a remove -d "Remove attachment"
complete -c kpass -n "__fish_seen_subcommand_from attach; and not __fish_seen_subcommand_from ls add remove extract" -a extract -d "Extract attachment"

# completion subcommand
complete -c kpass -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"

# Dynamic entry completion (uses cache file)
function __kpass_entries
    (commandline -poc) __complete entries "" "" "$argv" 2>/dev/null
end
complete -c kpass -n "__fish_seen_subcommand_from get g copy c edit e remove rm open history" -a "(__kpass_entries (commandline -ct))"
`
