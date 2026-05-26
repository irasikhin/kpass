package cli

// UserError is the user-facing error type — its message is printed to stderr
// and causes exit code 1. Wraps the Python KPassError contract.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }
