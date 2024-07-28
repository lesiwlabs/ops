package golang

import (
	"embed"

	"labs.lesiw.io/ops/clerkfs"
)

//go:embed .*
var f embed.FS

type Ops struct{}

func (op Ops) Sync() {
	if err := clerkfs.Add(f); err != nil {
		panic(err)
	}
	clerkfs.Apply()
}
