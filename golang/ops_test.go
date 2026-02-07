package golang

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func init() {
	GoModReplaceAllowed = true
}

func swap[T any](t *testing.T, ptr *T, val T) {
	t.Helper()
	old := *ptr
	*ptr = val
	t.Cleanup(func() { *ptr = old })
}

func TestTest(t *testing.T) {
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, "go", "gotestsum", "git")
	swap(t, &Builder, sh)
	swap(t, &Source, sh)
	swap(t, &GoTestSum, sh)

	Ops{}.Test()

	got := mock.Calls(m, "gotestsum")
	want := []mock.Call{
		{Args: []string{"gotestsum", "./...", "--", "-race"}},
	}
	if !cmp.Equal(want, got) {
		t.Errorf("gotestsum calls: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestLint(t *testing.T) {
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, "go", "git")
	swap(t, &Builder, sh)
	swap(t, &Source, sh)

	Ops{}.Lint()

	// Lint now only checks for go.mod replace directives (if not allowed)
	// No external linter commands are executed
}

func TestCov(t *testing.T) {
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, "go", "gotestsum", "git")
	swap(t, &Builder, sh)
	swap(t, &Source, sh)

	Ops{}.Cov()

	got := mock.Calls(m, "go")
	if len(got) < 2 {
		t.Fatalf("expected at least 2 go calls, got %d", len(got))
	}
	if got[0].Args[0] != "go" || got[0].Args[1] != "test" {
		t.Errorf("first call should be go test, got %v", got[0].Args)
	}
	if got[1].Args[0] != "go" || got[1].Args[1] != "tool" {
		t.Errorf("second call should be go tool, got %v", got[1].Args)
	}
}
