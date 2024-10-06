package golang

import (
	"sync"

	"labs.lesiw.io/ops/git"
	"lesiw.io/cmdio"
)

type Ops struct{}

var Runner = sync.OnceValue(func() *cmdio.Runner {
	if rnr, err := git.WorktreeRunner(); err != nil {
		panic(err)
	} else {
		return rnr
	}
})

func (Ops) Test() {
	Runner().MustRun(GoTestSum(), "./...", "--", "-race")
}

func (Ops) Lint() {
	Runner().MustRun(GolangCi(), "run")
}

func (Ops) Cov() {
	dir := Runner().MustGet("mktemp", "-d").Out
	defer Runner().Run("rm", "-rf", dir)
	Runner().MustRun("go", "test", "-coverprofile", dir+"/cover.out", "./...")
	Runner().MustRun("go", "tool", "cover", "-html="+dir+"/cover.out")
}
