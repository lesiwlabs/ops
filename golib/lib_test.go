package golib

import (
	"context"
	"slices"
	"testing"

	"labs.lesiw.io/ops/golang"
	"labs.lesiw.io/ops/internal/test"
	"lesiw.io/cmdio"
)

func TestCheckRunsOnce(t *testing.T) {
	defer clear(test.Uniq)

	cdr := new(test.EchoCdr)
	golang.Runner = cmdio.NewRunner(context.Background(), nil, cdr)

	for range 3 {
		Ops{}.Check()
	}

	var lintcmds int
	for _, cmd := range *cdr {
		if slices.Equal(cmd, []string{"[which golangci-lint]", "run"}) {
			lintcmds++
		}
	}
	if got, want := lintcmds, 1; got != want {
		t.Errorf("golangci-lint runs: %d, want %d", got, want)
	}
}
