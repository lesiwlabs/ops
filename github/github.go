package github

import (
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
)

type Ops struct{}

var Runner = sys.Runner()
var Repo string
var Secrets map[string]string

func (Ops) Secrets() {
	if Repo == "" {
		panic("github repo not set")
	}
	for k, v := range Secrets {
		cmdio.MustPipe(
			Runner.Command("spkez", "get", v),
			Runner.Command("gh", "-R", Repo, "secret", "set", k),
		)
	}
}
