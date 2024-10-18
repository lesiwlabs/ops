package goapp

import (
	"strings"

	"labs.lesiw.io/ops/git"
	"labs.lesiw.io/ops/golang"
	"lesiw.io/cmdio"
)

type Target struct {
	Goos   string
	Goarch string
	Unames string
	Unamer string
}

var Targets = []Target{
	{"linux", "386", "linux", "i386"},
	{"linux", "amd64", "linux", "x86_64"},
	{"linux", "arm", "linux", "armv7l"},
	{"linux", "arm64", "linux", "aarch64"},
	{"darwin", "amd64", "darwin", "x86_64"},
	{"darwin", "arm64", "darwin", "arm64"},
}

type Ops struct{ golang.Ops }

var Name string
var Versionfile = "version.txt"

func (op Ops) Build() {
	if Name == "" {
		panic("no app name given")
	}
	op.Clean()
	op.Lint()
	op.Test()
	for _, t := range Targets {
		golang.Runner().WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun(
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames+"-"+t.Unamer, ".",
		)
	}
	cmdio.MustPipe(
		golang.Runner().Command("tar", "-cf", "-", "out/"),
		git.Runner.Command("tar", "-xf", "-"),
	)
}

func (Ops) Clean() {
	git.Runner.MustRun("rm", "-rf", "out")
	git.Runner.MustRun("mkdir", "out")
}

func (Ops) Bump() {
	bump := cmdio.MustGetPipe(
		git.Runner.Command("curl", "lesiw.io/bump"),
		git.Runner.Command("sh"),
	).Out
	curVersion := git.Runner.MustGet("cat", Versionfile).Out
	version := cmdio.MustGetPipe(
		strings.NewReader(curVersion+"\n"),
		git.Runner.Command(bump, "-s", "1"),
		git.Runner.Command("tee", Versionfile),
	).Out
	git.Runner.MustRun("git", "add", Versionfile)
	git.Runner.MustRun("git", "commit", "-m", version)
	git.Runner.MustRun("git", "tag", version)
	git.Runner.MustRun("git", "push")
	git.Runner.MustRun("git", "push", "--tags")
}
