package golang

import (
	"lesiw.io/cmdio/sys"
)

type Ops struct{}

var Runner = sys.Runner()

func (Ops) Test() {
	Runner.MustRun(GoTestSum(), "./...", "--", "-race")
}

func (Ops) Lint() {
	Runner.MustRun(GolangCi(), "run")
}

func (Ops) Cov() {
	dir := Runner.MustGet("mktemp", "-d").Out
	defer Runner.Run("rm", "-rf", dir)
	Runner.MustRun("go", "test", "-coverprofile", dir+"/cover.out", "./...")
	Runner.MustRun("go", "tool", "cover", "-html="+dir+"/cover.out")
}
