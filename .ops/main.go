package main

import (
	"os"

	"labs.lesiw.io/ops/golang"
	"labs.lesiw.io/ops/golib"
	"lesiw.io/ops"
)

type Ops struct{ golib.Ops }

func main() {
	golang.GoModReplaceAllowed = true
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "check")
	}
	ops.Handle(Ops{})
}
