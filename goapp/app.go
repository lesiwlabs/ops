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
var BuildBox = sys.Box()
var LocalBox = sys.Box()

func (op Ops) Build() {
	op.Clean()
	op.Lint()
	op.Test()
	op.Race()
	for _, t := range Targets {
		sys.WithEnv(BuildBox, map[string]string{
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
	BuildBox.MustRun("rm", "-rf", "out")
	BuildBox.MustRun("mkdir", "out")
}

func (op Ops) Lint() {
	BuildBox.MustRun(golang.GolangCi(), "run")
}

func (op Ops) Test() {
	BuildBox.MustRun(golang.GoTestSum(), "./...")
}

func (op Ops) Race() {
	BuildBox.MustRun("go", "build", "-race", "-o", "/dev/null")
}

func (op Ops) Bump() {
	bump := cmdio.MustGetPipe(
		LocalBox.Command("curl", "lesiw.io/bump"),
		LocalBox.Command("sh"),
	).Output
	curVersion := LocalBox.MustGet("cat", Versionfile).Output
	version := cmdio.MustGetPipe(
		strings.NewReader(curVersion+"\n"),
		LocalBox.Command(bump, "-s", "1"),
		LocalBox.Command("tee", Versionfile),
	).Output
	LocalBox.MustRun("git", "add", Versionfile)
	LocalBox.MustRun("git", "commit", "-m", version)
	LocalBox.MustRun("git", "push")
}
