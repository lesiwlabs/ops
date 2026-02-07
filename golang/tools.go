package golang

import (
	"context"
	"fmt"
	"sync"

	"lesiw.io/command"
)

func installGotestsum(ctx context.Context, sh *command.Sh) error {
	return sh.Exec(ctx, "go", "install", "gotest.tools/gotestsum@latest")
}

var GoTestSum = func() *command.Sh {
	sh := Builder
	var install = sync.OnceValue(func() error {
		ctx := context.Background()
		err := command.Do(ctx, sh.Unshell(), "gotestsum", "--version")
		if command.NotFound(err) {
			return installGotestsum(ctx, sh)
		}
		return err
	})

	sh.HandleFunc("gotestsum", func(ctx context.Context, args ...string) command.Buffer {
		if err := install(); err != nil {
			return command.Fail(&command.Error{
				Err:  fmt.Errorf("failed to install gotestsum: %w", err),
				Code: 1,
			})
		}
		return sh.Unshell().Command(ctx, args...)
	})
	return sh
}()
