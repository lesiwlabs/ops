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

func (a Actions) Build() {
	a.Clean()
	a.Lint()
	a.Test()
	a.Race()
	for _, t := range Targets {
		box := sys.Env(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		})
		box.MustRun("go", "build", "-o", "/dev/null")
	}
}

func (a Actions) Clean() {
	sys.MustRun("rm", "-rf", "out")
	sys.MustRun("mkdir", "out")
}

func (a Actions) Lint() {
	sys.MustRun(golang.GolangCi(), "run")
	sys.MustRun("go", "run", "github.com/bobg/mingo/cmd/mingo@latest", "-check")
}

func (a Actions) Test() {
	sys.MustRun(golang.GoTestSum(), "./...")
}

func (a Actions) Race() {
	sys.MustRun("go", "build", "-race", "-o", "/dev/null")
}

func (a Actions) Bump() {
	bump := cmdio.MustGetPipe(
		sys.Command("curl", "lesiw.io/bump"),
		sys.Command("sh"),
	).Output
	version := cmdio.MustGetPipe(
		sys.Command("git", "describe", "--abbrev=0", "--tags"),
		sys.Command(bump, "-s", "1"),
	).Output
	sys.MustRun("git", "tag", version)
	sys.MustRun("git", "push")
	sys.MustRun("git", "push", "--tags")
}
