package github

import (
	"context"
	"fmt"
	"io"

	"lesiw.io/command"
	"lesiw.io/command/sys"
)

type Ops struct{}

var Shell = command.Shell(sys.Machine(), "spkez", "gh")
var Repo string
var Secrets map[string]string

func (Ops) Secrets(ctx context.Context) error {
	if Repo == "" {
		return fmt.Errorf("github repo not set")
	}
	for k, v := range Secrets {
		_, err := io.Copy(
			command.NewWriter(ctx, Shell,
				"gh", "-R", Repo, "secret", "set", k),
			command.NewReader(ctx, Shell, "spkez", "get", v),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
