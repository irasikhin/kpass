package cli

import (
	"github.com/irasikhin/kpass/internal/pwgen"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// selectPassword resolves the inline / stdin / generate / prompt branches in
// the order: explicit > stdin > generate > interactive prompt. Errors when
// more than one explicit source is set.
func (f passwordFlags) selectPassword(c *ctx, prompt string, confirm bool) (string, error) {
	provided := 0
	if f.Password != nil {
		provided++
	}
	if f.PasswordStdin {
		provided++
	}
	if f.Generate {
		provided++
	}
	if provided > 1 {
		return "", &UserError{Msg: "Choose only one of --password, --password-stdin, or --generate."}
	}
	if f.Password != nil {
		return *f.Password, nil
	}
	if f.PasswordStdin {
		return runtimex.ReadSecretFromStdin(c.in)
	}
	if f.Generate {
		lower, upper, digits, symbols := resolveCharsetFlags(f.Lower, f.Upper, f.Digits, f.Symbols,
			f.NoLower, f.NoUpper, f.NoDigits, f.NoSymbols)
		symChars := ""
		if f.SymbolChars != nil {
			symChars = *f.SymbolChars
		}
		return pwgen.Generate(f.Length, lower, upper, digits, symbols, symChars)
	}
	return runtimex.PromptSecret(prompt, confirm)
}
