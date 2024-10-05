package golang

import (
	"lesiw.io/cmdio/sys"
)

type Ops struct{}

var Runner = sys.Runner()

func (Ops) Test() {
	Runner.MustRun(GoTestSum(), "./...", "--", "-race")
}
