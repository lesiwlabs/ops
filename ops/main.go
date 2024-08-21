package main

import (
	"labs.lesiw.io/ops/golib"
	"lesiw.io/ops"
)

type Ops struct{ golib.Ops }

func main() { ops.Handle(Ops{}) }
