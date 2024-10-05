package golang

import (
	"embed"

	"labs.lesiw.io/ops/clerkfs"
)

//go:embed .*
var f embed.FS

func (Ops) Sync() {
	if err := clerkfs.Add(f); err != nil {
		panic(err)
	}
	clerkfs.Apply()
}
