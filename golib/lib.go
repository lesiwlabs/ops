package golib

import (
	"labs.lesiw.io/ci/golang"
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

type Actions struct {
	golang.Actions
}

var Name string
var Box *cmdio.Box = sys.Box()

func (a Actions) Build() {
	a.Clean()
	a.Lint()
	a.Test()
	a.Race()
	for _, t := range Targets {
		sys.WithEnv(Box, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun("go", "build", "-o", "/dev/null")
	}
}

func (a Actions) Clean() {
	Box.MustRun("rm", "-rf", "out")
	Box.MustRun("mkdir", "out")
}

func (a Actions) Lint() {
	Box.MustRun(golang.GolangCi(), "run")
	Box.MustRun("go", "run", "github.com/bobg/mingo/cmd/mingo@latest", "-check")
}

func (a Actions) Test() {
	Box.MustRun(golang.GoTestSum(), "./...")
}

func (a Actions) Race() {
	Box.MustRun("go", "build", "-race", "-o", "/dev/null")
}

func (a Actions) Bump() {
	bump := cmdio.MustGetPipe(
		Box.Command("curl", "lesiw.io/bump"),
		Box.Command("sh"),
	).Output
	version := cmdio.MustGetPipe(
		Box.Command("git", "describe", "--abbrev=0", "--tags"),
		Box.Command(bump, "-s", "1"),
	).Output
	Box.MustRun("git", "tag", version)
	Box.MustRun("git", "push")
	Box.MustRun("git", "push", "--tags")
}
