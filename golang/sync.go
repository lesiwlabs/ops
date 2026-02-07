package golang

import (
	"context"
	"embed"
	"fmt"

	"labs.lesiw.io/ops/clerkfs"
	"lesiw.io/command"
	"lesiw.io/command/sys"
)

//go:embed .*
var f embed.FS

func (Ops) Sync() error {
	ctx := command.WithEnv(context.Background(), map[string]string{"PWD": ".ops"})
	sh := command.Shell(sys.Machine(), "go")
	err := sh.Exec(ctx, "go", "get", "-u", "all")
	if err != nil {
		return fmt.Errorf("failed to run go mod -u all: %w", err)
	}
	if err := clerkfs.Add(f); err != nil {
		return fmt.Errorf("could not add file to clerk: %w", err)
	}
	clerkfs.Apply() // FIXME: Can still panic.
	return nil
}
