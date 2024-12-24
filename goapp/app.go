package goapp

import (
	"strings"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
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
		golang.Builder().WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun(
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames+"-"+t.Unamer, ".",
		)
	}
	cmdio.MustPipe(
		golang.Builder().Command("tar", "-cf", "-", "out/"),
		golang.Source().Command("tar", "-xf", "-"),
	)
}

func (Ops) Clean() {
	golang.Source().MustRun("rm", "-rf", "out")
	golang.Source().MustRun("mkdir", "out")
}

func (Ops) Bump() {
	if _, err := golang.Source().Get("which", "bump"); err != nil {
		golang.Source().MustRun("go", "install", "lesiw.io/bump@latest")
	}
	curVersion := golang.Builder().MustGet("cat", Versionfile).Out
	version := cmdio.MustGetPipe(
		strings.NewReader(curVersion+"\n"),
		golang.Source().WithCommand("bump", sys.Runner()).
			Command("bump", "-s", "1"),
		golang.Source().Command("tee", Versionfile),
	).Out
	golang.Source().MustRun("git", "add", Versionfile)
	golang.Source().MustRun("git", "commit", "-m", version)
	golang.Source().MustRun("git", "tag", version)
	golang.Source().MustRun("git", "push")
	golang.Source().MustRun("git", "push", "--tags")
}
