package golang

import (
	"sync"

	"labs.lesiw.io/ops/git"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/x/busybox"
)

type Ops struct{}

var Runner = sync.OnceValue(func() *cmdio.Runner {
	if rnr, err := git.WorktreeRunner(); err != nil {
		panic(err)
	} else {
		return rnr
	}
})
var Busybox = sync.OnceValue(func() *cmdio.Runner {
	if rnr, err := busybox.Runner(); err != nil {
		panic(err)
	} else {
		return rnr
	}
})
var GoModReplaceAllowed bool

func (Ops) Test() {
	Runner().MustRun(GoTestSum(), "./...", "--", "-race")
}

func (Ops) Lint() {
	Runner().MustRun(GolangCi(), "run")
	if !GoModReplaceAllowed {
		r := Busybox().MustGet("find", ".", "-type", "f", "-name", "go.mod",
			"-exec",
			"grep", "-n", "^replace", "go.mod", "/dev/null", ";")
		if r.Out != "" {
			panic("replace directive found in go.mod\n" + r.Out)
		}
	}
}

func (Ops) Cov() {
	dir := Busybox().MustGet("mktemp", "-d").Out
	defer Busybox().Run("rm", "-rf", dir)
	Runner().MustRun("go", "test", "-coverprofile", dir+"/cover.out", "./...")
	Runner().MustRun("go", "tool", "cover", "-html="+dir+"/cover.out")
}
