package golang

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"labs.lesiw.io/ops/internal/test"
	"lesiw.io/cmdio"
)

func TestTest(t *testing.T) {
	cdr := new(test.EchoCdr)
	Runner = cmdio.NewRunner(context.Background(), nil, cdr)

	Ops{}.Test()

	expectcdr := test.EchoCdr{
		{"which", "gotestsum"},
		{"[which gotestsum]", "./...", "--", "-race"},
	}
	if got, want := *cdr, expectcdr; !cmp.Equal(got, want) {
		t.Errorf("cmds: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestLint(t *testing.T) {
	cdr := new(test.EchoCdr)
	Runner = cmdio.NewRunner(context.Background(), nil, cdr)

	Ops{}.Lint()

	expectcdr := test.EchoCdr{
		{"which", "golangci-lint"},
		{"[which golangci-lint]", "run"},
	}
	if got, want := *cdr, expectcdr; !cmp.Equal(got, want) {
		t.Errorf("cmds: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestCov(t *testing.T) {
	cdr := new(test.EchoCdr)
	Runner = cmdio.NewRunner(context.Background(), nil, cdr)

	Ops{}.Cov()

	expectcdr := test.EchoCdr{
		{"mktemp", "-d"},
		{"go", "test", "-coverprofile", "[mktemp -d]/cover.out", "./..."},
		{"go", "tool", "cover", "-html=[mktemp -d]/cover.out"},
		{"rm", "-rf", "[mktemp -d]"},
	}
	if got, want := *cdr, expectcdr; !cmp.Equal(got, want) {
		t.Errorf("cmds: -want +got\n%s", cmp.Diff(want, got))
	}
}
