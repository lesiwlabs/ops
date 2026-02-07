package github

import (
	"context"
	"io"

	"lesiw.io/command"
	"lesiw.io/command/sys"
)

type Ops struct{}

var Shell = command.Shell(sys.Machine(), "spkez", "gh")
var Repo string
var Secrets map[string]string

func (Ops) Secrets() {
	if Repo == "" {
		panic("github repo not set")
	}
	ctx := context.Background()
	for k, v := range Secrets {
		_, err := io.Copy(
			command.NewWriter(ctx, Shell,
				"gh", "-R", Repo, "secret", "set", k),
			command.NewReader(ctx, Shell, "spkez", "get", v),
		)
		if err != nil {
			panic(err)
		}
	}
}
