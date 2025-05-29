package golib

import (
	"fmt"
	"runtime"
	"sync"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
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
		golang.Builder().WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun("go", "build", "-o", "/dev/null", "./...")
	}
}

func (op Ops) Lint() {
	op.Ops.Lint()
	if runtime.GOOS != "windows" {
		golang.Builder().MustRun("go", "run",
			"github.com/bobg/mingo/cmd/mingo@latest", "-check")
	}
}

func (op Ops) Bump() {
	op.Check()
	if _, err := golang.Source().Get("which", "bump"); err != nil {
		golang.Builder().MustRun("go", "install", "lesiw.io/bump@latest")
	}
	version := cmdio.MustGetPipe(
		golang.Source().Command("git", "describe", "--abbrev=0", "--tags"),
		golang.Source().WithCommand("bump", sys.Runner()).
			Command("bump", "-s", "1"),
	).Out
	golang.Source().MustRun("git", "tag", version)
	golang.Source().MustRun("git", "push")
	golang.Source().MustRun("git", "push", "--tags")
}

func (Ops) ProxyPing() {
	var ref string
	tag, err := golang.Source().Get("git", "describe", "--exact-match",
		"--tags")
	if err == nil {
		ref = tag.Out
	} else {
		ref = golang.Source().MustGet("git", "rev-parse", "HEAD").Out
	}
	mod := golang.Builder().MustGet("go", "list", "-m").Out
	golang.Builder().MustRun("go", "list", "-m",
		fmt.Sprintf("%s@%s", mod, ref))
}
