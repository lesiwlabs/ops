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
		golang.Busybox().Command("tar", "-xf", "-"),
	)
}

func (Ops) Clean() {
	golang.Busybox().MustRun("rm", "-rf", "out")
	golang.Busybox().MustRun("mkdir", "out")
}

func (Ops) Bump() {
	if _, err := golang.Runner().Get("which", "bump"); err != nil {
		golang.Runner().MustRun("go", "install", "lesiw.io/bump@latest")
	}
	curVersion := golang.Busybox().MustGet("cat", Versionfile).Out
	version := cmdio.MustGetPipe(
		strings.NewReader(curVersion+"\n"),
		golang.Runner().Command("bump", "-s", "1"),
		golang.Busybox().Command("tee", Versionfile),
	).Out
	git.Runner().MustRun("add", Versionfile)
	git.Runner().MustRun("commit", "-m", version)
	git.Runner().MustRun("tag", version)
	git.Runner().MustRun("push")
	git.Runner().MustRun("push", "--tags")
}
