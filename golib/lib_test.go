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
	cdr := new(testcdr)
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

type testcdr [][]string

func (c *testcdr) Command(
	_ context.Context, env map[string]string, args ...string,
) io.ReadWriter {
	*c = append(*c, args)
	return bytes.NewBufferString(fmt.Sprintf("%v\n", args))
}
