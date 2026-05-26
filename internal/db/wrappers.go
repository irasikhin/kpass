package db

import (
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

func newBoolWrapper(v bool) w.BoolWrapper {
	return w.NewBoolWrapper(v)
}
