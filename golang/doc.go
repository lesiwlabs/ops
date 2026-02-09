package golang

import (
	"context"
	"fmt"
	"strings"

	"lesiw.io/command"
)

func (Ops) Doc(ctx context.Context) error {
	mod, err := Build.Read(ctx, "go", "list", "-m")
	if err != nil {
		return fmt.Errorf("detect module: %w", err)
	}
	out, err := command.Read(ctx, goreadme,
		"-skip-sub-packages")
	if err != nil {
		return fmt.Errorf("goreadme: %w", err)
	}
	content := "# " + mod + " " +
		"[![Go Reference]" +
		"(https://pkg.go.dev/badge/" + mod + ".svg)]" +
		"(https://pkg.go.dev/" + mod + ")\n" +
		out[strings.Index(out, "\n")+1:] + "\n"
	return Build.WriteFile(
		ctx, "docs/README.md", []byte(content))
}
