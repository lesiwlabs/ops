package golib

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"testing"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/cmdio"
)

func TestCheckRunsOnce(t *testing.T) {
	bc := new(testcmdr)
	golang.Runner = cmdio.NewRunner(context.Background(), nil, bc)

	for range 3 {
		Ops{}.Check()
	}

	var lintcmds int
	for _, cmd := range bc.cmds {
		if slices.Equal(cmd, []string{"[which golangci-lint]", "run"}) {
			lintcmds++
		}
	}
	if got, want := lintcmds, 1; got != want {
		t.Errorf("golangci-lint runs: %d, want %d", got, want)
	}
}

type testcmdr struct {
	cmds [][]string
}

func (t *testcmdr) Command(
	_ context.Context, env map[string]string, args ...string,
) io.ReadWriter {
	t.cmds = append(t.cmds, args)
	return bytes.NewBufferString(fmt.Sprintf("%v\n", args))
}
