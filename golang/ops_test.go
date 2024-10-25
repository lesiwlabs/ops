package golang

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"labs.lesiw.io/ops/internal/test"
	"lesiw.io/cmdio"
)

func init() {
	GoModReplaceAllowed = true
}

func TestTest(t *testing.T) {
	defer clear(test.Uniq)

	cdr := new(test.EchoCdr)
	rnr := func() *cmdio.Runner {
		return new(cmdio.Runner).WithCommander(cdr)
	}
	Builder = rnr
	Source = rnr
	GoTestSum = rnr
	GolangCi = rnr

	Ops{}.Test()

	expectcdr := test.EchoCdr{
		{"./...", "--", "-race"},
	}
	if got, want := *cdr, expectcdr; !cmp.Equal(got, want) {
		t.Errorf("cmds: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestLint(t *testing.T) {
	defer clear(test.Uniq)

	cdr := new(test.EchoCdr)
	rnr := func() *cmdio.Runner {
		return new(cmdio.Runner).WithCommander(cdr)
	}
	Builder = rnr
	Source = rnr
	GoTestSum = rnr
	GolangCi = rnr

	Ops{}.Lint()

	expectcdr := test.EchoCdr{
		{"run"},
	}
	if got, want := *cdr, expectcdr; !cmp.Equal(got, want) {
		t.Errorf("cmds: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestCov(t *testing.T) {
	defer clear(test.Uniq)

	cdr := new(test.EchoCdr)
	rnr := func() *cmdio.Runner {
		return new(cmdio.Runner).WithCommander(cdr)
	}
	Builder = rnr
	Source = rnr
	GoTestSum = rnr
	GolangCi = rnr

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
