package golang

import (
	"embed"
	"fmt"

	"labs.lesiw.io/ops/clerkfs"
	"lesiw.io/cmdio/sys"
)

//go:embed .*
var f embed.FS

func (Ops) Sync() error {
	err := sys.Runner().WithEnv(map[string]string{"PWD": ".ops"}).
		Run("go", "get", "-u", "all")
	if err != nil {
		return fmt.Errorf("failed to run go mod -u all: %w", err)
	}
	if err := clerkfs.Add(f); err != nil {
		return fmt.Errorf("could not add file to clerk: %w", err)
	}
	clerkfs.Apply() // FIXME: Can still panic.
	return nil
}
