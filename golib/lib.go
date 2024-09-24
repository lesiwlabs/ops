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
var BuildBox = sys.Box()
var LocalBox = sys.Box()
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

func (op Ops) Compile() {
	for _, t := range Targets {
		sys.WithEnv(BuildBox, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun("go", "build", "-o", "/dev/null")
	}
}

func (op Ops) Clean() {
	BuildBox.MustRun("rm", "-rf", "out")
	BuildBox.MustRun("mkdir", "out")
}

func (op Ops) Lint() {
	BuildBox.MustRun(golang.GolangCi(), "run")
	if runtime.GOOS != "windows" {
		BuildBox.MustRun("go", "run", "github.com/bobg/mingo/cmd/mingo@latest",
			"-check")
	}
}

func (op Ops) Test() {
	BuildBox.MustRun(golang.GoTestSum(), "./...")
}

func (op Ops) Race() {
	BuildBox.MustRun("go", "build", "-race", "-o", "/dev/null")
}

func (op Ops) Bump() {
	op.Check()
	bump := cmdio.MustGetPipe(
		LocalBox.Command("curl", "lesiw.io/bump"),
		LocalBox.Command("sh"),
	).Output
	version := cmdio.MustGetPipe(
		LocalBox.Command("git", "describe", "--abbrev=0", "--tags"),
		LocalBox.Command(bump, "-s", "1"),
	).Output
	LocalBox.MustRun("git", "tag", version)
	LocalBox.MustRun("git", "push")
	LocalBox.MustRun("git", "push", "--tags")
}

func (op Ops) ProxyPing() {
	var ref string
	tag, err := LocalBox.Get("git", "describe", "--exact-match", "--tags")
	if err == nil {
		ref = tag.Output
	} else {
		ref = LocalBox.MustGet("git", "rev-parse", "HEAD").Output
	}
	mod := LocalBox.MustGet("go", "list", "-m").Output
	LocalBox.MustRun("go", "list", "-m", fmt.Sprintf("%s@%s", mod, ref))
}
