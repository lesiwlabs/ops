package golang

import (
	"embed"

	"labs.lesiw.io/ci/clerkfs"
)

//go:embed .*
var f embed.FS

type Actions struct{}

func (a Actions) Sync() {
	if err := clerkfs.Add(f); err != nil {
		panic(err)
	}
	clerkfs.Apply()
}
