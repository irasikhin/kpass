package cli

import (
	"github.com/irasikhin/kpass/internal/pwgen"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// passwordOpts is the parsed form of the inline / stdin / generate password
// flags. Constructed by passwordFlags.asOpts on each command struct and used
// by selectPassword to decide where to source the password from.
type passwordOpts struct {
	provided      *string
	passwordStdin bool
	generate      bool
	length        int
	lower         bool
	upper         bool
	digits        bool
	symbols       bool
	noLower       bool
	noUpper       bool
	noDigits      bool
	noSymbols     bool
	symbolChars   *string
}

func (o passwordOpts) selectPassword(c *ctx, prompt string, confirm bool) (string, error) {
	provided := 0
	if o.provided != nil {
		provided++
	}
	if o.passwordStdin {
		provided++
	}
	if o.generate {
		provided++
	}
	if provided > 1 {
		return "", &UserError{Msg: "Choose only one of --password, --password-stdin, or --generate."}
	}
	if o.provided != nil {
		return *o.provided, nil
	}
	if o.passwordStdin {
		return runtimex.ReadSecretFromStdin(c.in)
	}
	if o.generate {
		lower, upper, digits, symbols := resolveCharsetFlags(o.lower, o.upper, o.digits, o.symbols,
			o.noLower, o.noUpper, o.noDigits, o.noSymbols)
		symChars := ""
		if o.symbolChars != nil {
			symChars = *o.symbolChars
		}
		return pwgen.Generate(o.length, lower, upper, digits, symbols, symChars)
	}
	return runtimex.PromptSecret(prompt, confirm)
}
