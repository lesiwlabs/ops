package golib

import (
	"fmt"
	"runtime"
	"sync"

	"labs.lesiw.io/ops/git"
	"labs.lesiw.io/ops/golang"
	"lesiw.io/cmdio"
)

type Target struct {
	Goos   string
	Goarch string
}

var Targets = []Target{
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "386"},
	{"windows", "arm"},
	{"windows", "amd64"},
	{"plan9", "386"},
	{"plan9", "arm"},
	{"plan9", "amd64"},
}

type Ops struct{ golang.Ops }

var checkOnce sync.Once

func (op Ops) Check() {
	checkOnce.Do(func() {
		op.Lint()
		op.Test()
		op.Compile()
	})
}

func (op Ops) Build() {
	op.Check()
}

func (Ops) Compile() {
	for _, t := range Targets {
		golang.Runner().WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun("go", "build", "-o", "/dev/null", "./...")
	}
}

func (op Ops) Lint() {
	op.Ops.Lint()
	if runtime.GOOS != "windows" {
		golang.Runner().MustRun("go", "run",
			"github.com/bobg/mingo/cmd/mingo@latest", "-check")
	}
}

func (op Ops) Bump() {
	op.Check()
	if _, err := golang.Busybox().Get("which", "bump"); err != nil {
		golang.Busybox().MustRun("go", "install", "lesiw.io/bump@latest")
	}
	version := cmdio.MustGetPipe(
		git.Runner().Command("describe", "--abbrev=0", "--tags"),
		golang.Busybox().Command("bump", "-s", "1"),
	).Out
	git.Runner().MustRun("tag", version)
	git.Runner().MustRun("push")
	git.Runner().MustRun("push", "--tags")
}

func (Ops) ProxyPing() {
	var ref string
	tag, err := git.Runner().Get("describe", "--exact-match", "--tags")
	if err == nil {
		ref = tag.Out
	} else {
		ref = git.Runner().MustGet("rev-parse", "HEAD").Out
	}
	mod := golang.Runner().MustGet("go", "list", "-m").Out
	golang.Runner().MustRun("go", "list", "-m", fmt.Sprintf("%s@%s", mod, ref))
}
