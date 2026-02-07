package golang

import (
	"context"
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

func setupMock(t *testing.T, programs ...string) *mock.Machine {
	t.Helper()
	ctx := context.Background()
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, programs...)
	if err := sh.WriteFile(ctx, "go.mod",
		[]byte("module test\n")); err != nil {
		t.Fatal(err)
	}
	swap(t, &Builder, sh)
	swap(t, &Source, sh)
	return m
}

func TestTest(t *testing.T) {
	m := setupMock(t, "go", "git")

	err := Ops{}.Test()
	if err != nil {
		t.Fatal(err)
	}

	got := mock.Calls(m, "go")
	want := []mock.Call{
		{
			Args: []string{"go", "-C", ".", "test",
				"-count=1", "-shuffle=on", "./..."},
			Env: map[string]string{"CGO_ENABLED": "0"},
		},
		{
			Args: []string{"go", "-C", ".", "test",
				"-count=1", "-shuffle=on", "-race", "./..."},
			Env: map[string]string{"CGO_ENABLED": "1"},
		},
	}
	if !cmp.Equal(want, got) {
		t.Errorf("go calls: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestLint(t *testing.T) {
	setupMock(t, "go", "git")

	err := Ops{}.Lint()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCov(t *testing.T) {
	m := setupMock(t, "go", "git")

	err := Ops{}.Cov()
	if err != nil {
		t.Fatal(err)
	}

	got := mock.Calls(m, "go")
	want := []mock.Call{
		{Args: []string{"go", "test", "-coverprofile",
			got[0].Args[3], "./..."}},
		{Args: []string{"go", "tool", "cover",
			"-html=" + got[0].Args[3]}},
	}
	if !cmp.Equal(want, got) {
		t.Errorf("go calls: -want +got\n%s", cmp.Diff(want, got))
	}
}

func TestAnalyzers(t *testing.T) {
	got := Analyzers()
	if len(got) == 0 {
		t.Error("Analyzers() returned empty list")
	}
	names := make(map[string]bool)
	for _, a := range got {
		if names[a.Name] {
			t.Errorf("duplicate analyzer: %s", a.Name)
		}
		names[a.Name] = true
	}
}

func TestFixAnalyzers(t *testing.T) {
	got := FixAnalyzers()
	if len(got) == 0 {
		t.Error("FixAnalyzers() returned empty list")
	}
	all := make(map[string]bool)
	for _, a := range Analyzers() {
		all[a.Name] = true
	}
	for _, a := range got {
		if !all[a.Name] {
			t.Errorf("fix analyzer %q not in Analyzers()", a.Name)
		}
	}
}
