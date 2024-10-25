package golang

import (
	"sync"

	"labs.lesiw.io/ops/git"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
	"lesiw.io/cmdio/x/busybox"
)

type Ops struct{}

var Source = sync.OnceValue(func() *cmdio.Runner {
	if rnr, err := busybox.Runner(); err != nil {
		panic(err)
	} else {
		return rnr.WithCommand("git",
			rnr.WithCommander(sys.Runner().Commander))
	}
})
var Builder = sync.OnceValue(func() *cmdio.Runner {
	if rnr, err := git.WorktreeRunner(Source()); err != nil {
		panic(err)
	} else {
		return rnr.WithCommand("go",
			rnr.WithCommander(sys.Runner().Commander))
	}
})
var GoModReplaceAllowed bool

func (Ops) Test() {
	GoTestSum().MustRun("./...", "--", "-race")
}

func (Ops) Lint() {
	GolangCi().MustRun("run")
	if !GoModReplaceAllowed {
		r := Builder().MustGet("find", ".", "-type", "f", "-name", "go.mod",
			"-exec",
			"grep", "-n", "^replace", "go.mod", "/dev/null", ";")
		if r.Out != "" {
			panic("replace directive found in go.mod\n" + r.Out)
		}
	}
}

func (Ops) Cov() {
	dir := Builder().MustGet("mktemp", "-d").Out
	defer Builder().Run("rm", "-rf", dir)
	Builder().MustRun("go", "test", "-coverprofile", dir+"/cover.out", "./...")
	Builder().MustRun("go", "tool", "cover", "-html="+dir+"/cover.out")
}
