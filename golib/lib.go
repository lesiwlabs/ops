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

var Name string
var BuildRnr = sys.Runner()
var LocalRnr = sys.Runner()
var checkOnce sync.Once

func (op Ops) Check() {
	checkOnce.Do(func() {
		op.Clean()
		op.Lint()
		op.Test()
		op.Race()
		op.Compile()
	})
}

func (op Ops) Build() {
	op.Check()
}

func (Ops) Compile() {
	for _, t := range Targets {
		BuildRnr.WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun("go", "build", "-o", "/dev/null", "./...")
	}
}

func (Ops) Clean() {
	BuildRnr.MustRun("rm", "-rf", "out")
	BuildRnr.MustRun("mkdir", "out")
}

func (Ops) Lint() {
	BuildRnr.MustRun(golang.GolangCi(), "run")
	if runtime.GOOS != "windows" {
		BuildRnr.MustRun("go", "run", "github.com/bobg/mingo/cmd/mingo@latest",
			"-check")
	}
}

func (Ops) Test() {
	BuildRnr.MustRun(golang.GoTestSum(), "./...")
}

func (Ops) Race() {
	BuildRnr.MustRun("go", "build", "-o", "/dev/null", "-race", "./...")
}

func (op Ops) Bump() {
	op.Check()
	bump := cmdio.MustGetPipe(
		LocalRnr.Command("curl", "lesiw.io/bump"),
		LocalRnr.Command("sh"),
	).Out
	version := cmdio.MustGetPipe(
		LocalRnr.Command("git", "describe", "--abbrev=0", "--tags"),
		LocalRnr.Command(bump, "-s", "1"),
	).Out
	LocalRnr.MustRun("git", "tag", version)
	LocalRnr.MustRun("git", "push")
	LocalRnr.MustRun("git", "push", "--tags")
}

func (Ops) ProxyPing() {
	var ref string
	tag, err := LocalRnr.Get("git", "describe", "--exact-match", "--tags")
	if err == nil {
		ref = tag.Out
	} else {
		ref = LocalRnr.MustGet("git", "rev-parse", "HEAD").Out
	}
	mod := LocalRnr.MustGet("go", "list", "-m").Out
	LocalRnr.MustRun("go", "list", "-m", fmt.Sprintf("%s@%s", mod, ref))
}
