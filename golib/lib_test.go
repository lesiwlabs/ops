package golib

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"testing"

	"lesiw.io/cmdio"
)

func TestCheckRunsOnce(t *testing.T) {
	bc := new(testcmdr)
	BuildRnr = cmdio.NewRunner(context.Background(), nil, bc)

	for range 3 {
		Ops{}.Check()
	}

	var lintcmds int
	for _, cmd := range bc.cmds {
		if slices.Equal(cmd, []string{"golangci-lint", "run"}) {
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
	if slices.Equal(args, []string{"which", "golangci-lint"}) {
		return reply{args, bytes.NewBufferString("golangci-lint")}
	}
	return reply{args, bytes.NewBufferString(fmt.Sprintf("%v\n", args))}
}

type reply struct {
	arg []string
	io.ReadWriter
}

func (r reply) String() string {
	return fmt.Sprintf("%v", r.arg)
}
