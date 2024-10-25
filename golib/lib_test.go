package golib

import (
	"slices"
	"testing"

	"labs.lesiw.io/ops/golang"
	"labs.lesiw.io/ops/internal/test"
	"lesiw.io/cmdio"
)

func init() {
	golang.GoModReplaceAllowed = true
}

func TestCheckRunsOnce(t *testing.T) {
	defer clear(test.Uniq)

	cdr := new(test.EchoCdr)
	rnr := func() *cmdio.Runner {
		return new(cmdio.Runner).WithCommander(cdr)
	}
	golang.Builder = rnr
	golang.Source = rnr
	golang.GoTestSum = rnr
	golang.GolangCi = rnr

	for range 3 {
		Ops{}.Check()
	}

	var lintcmds int
	for _, cmd := range *cdr {
		if slices.Equal(cmd, []string{"run"}) {
			lintcmds++
		}
	}
	if got, want := lintcmds, 1; got != want {
		t.Errorf("golangci-lint runs: %d, want %d", got, want)
	}
}
