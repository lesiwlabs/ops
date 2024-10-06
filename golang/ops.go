package golang

import "labs.lesiw.io/ops/git"

type Ops struct{}

var Runner = git.WorktreeRunner()

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
