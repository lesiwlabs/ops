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

type Ops struct {
	golang.Ops
}

var Name string
var Versionfile = "version.txt"
var BuildRnr = sys.Runner()
var LocalRnr = sys.Runner()

func (op Ops) Build() {
	op.Clean()
	op.Lint()
	op.Test()
	op.Race()
	for _, t := range Targets {
		BuildRnr.WithEnv(map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		}).MustRun(
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames+"-"+t.Unamer, ".",
		)
	}
}

func (op Ops) Clean() {
	BuildRnr.MustRun("rm", "-rf", "out")
	BuildRnr.MustRun("mkdir", "out")
}

func (op Ops) Lint() {
	BuildRnr.MustRun(golang.GolangCi(), "run")
}

func (op Ops) Test() {
	BuildRnr.MustRun(golang.GoTestSum(), "./...")
}

func (op Ops) Race() {
	BuildRnr.MustRun("go", "build", "-race", "-o", "/dev/null")
}

func (op Ops) Bump() {
	bump := cmdio.MustGetPipe(
		LocalRnr.Command("curl", "lesiw.io/bump"),
		LocalRnr.Command("sh"),
	).Out
	curVersion := LocalRnr.MustGet("cat", Versionfile).Out
	version := cmdio.MustGetPipe(
		strings.NewReader(curVersion+"\n"),
		LocalRnr.Command(bump, "-s", "1"),
		LocalRnr.Command("tee", Versionfile),
	).Out
	LocalRnr.MustRun("git", "add", Versionfile)
	LocalRnr.MustRun("git", "commit", "-m", version)
	LocalRnr.MustRun("git", "tag", version)
	LocalRnr.MustRun("git", "push")
	LocalRnr.MustRun("git", "push", "--tags")
}
