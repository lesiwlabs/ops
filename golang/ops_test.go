package golang

import (
	"context"
	"strings"
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
	swap(t, &Build, sh)
	swap(t, &Local, sh)
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

var buffer = strings.NewReader

func TestPromoteSuccess(t *testing.T) {
	m := setupMock(t, "git", "gh")
	m.Return(buffer("next"),
		"git", "branch", "--show-current")
	m.Return(buffer("origin"),
		"git", "config", "--get", "branch.next.remote")
	m.Return(buffer("abc123"),
		"git", "rev-parse", "origin/main")
	m.Return(buffer("def456"),
		"git", "rev-parse", "next")
	m.Return(buffer("def456"), "gh", "run", "list",
		"--branch", "next", "--limit", "1",
		"--json", "headSha", "--jq", ".[0].headSha")
	m.Return(buffer("success"), "gh", "run", "list",
		"--branch", "next", "--limit", "1",
		"--json", "conclusion",
		"--jq", ".[0].conclusion")

	err := Ops{}.Promote()
	if err != nil {
		t.Fatalf("Promote() failed: %v", err)
	}

	gotGit := mock.Calls(m, "git")
	wantGit := []mock.Call{
		{Args: []string{"git", "branch", "--show-current"}},
		{Args: []string{
			"git", "config", "--get",
			"branch.next.remote",
		}},
		{Args: []string{"git", "fetch", "origin"}},
		{Args: []string{
			"git", "merge-base", "--is-ancestor",
			"origin/main", "next",
		}},
		{Args: []string{"git", "rev-parse", "origin/main"}},
		{Args: []string{"git", "rev-parse", "next"}},
		{Args: []string{
			"git", "push", "origin", "next:main",
		}},
		{Args: []string{
			"git", "fetch", "origin", "main:main",
		}},
	}
	if !cmp.Equal(wantGit, gotGit) {
		t.Errorf("git calls: -want +got\n%s",
			cmp.Diff(wantGit, gotGit))
	}

	gotGh := mock.Calls(m, "gh")
	wantGh := []mock.Call{
		{Args: []string{
			"gh", "run", "list",
			"--branch", "next", "--limit", "1",
			"--json", "headSha",
			"--jq", ".[0].headSha",
		}},
		{Args: []string{
			"gh", "run", "list",
			"--branch", "next", "--limit", "1",
			"--json", "conclusion",
			"--jq", ".[0].conclusion",
		}},
	}
	if !cmp.Equal(wantGh, gotGh) {
		t.Errorf("gh calls: -want +got\n%s",
			cmp.Diff(wantGh, gotGh))
	}
}

func TestPromoteCIFailed(t *testing.T) {
	m := setupMock(t, "git", "gh")
	m.Return(buffer("next"),
		"git", "branch", "--show-current")
	m.Return(buffer("origin"),
		"git", "config", "--get", "branch.next.remote")
	m.Return(buffer("abc123"),
		"git", "rev-parse", "origin/main")
	m.Return(buffer("def456"),
		"git", "rev-parse", "next")
	m.Return(buffer("def456"), "gh", "run", "list",
		"--branch", "next", "--limit", "1",
		"--json", "headSha", "--jq", ".[0].headSha")
	m.Return(buffer("PENDING"), "gh", "run", "list",
		"--branch", "next", "--limit", "1",
		"--json", "conclusion",
		"--jq", ".[0].conclusion")

	err := Ops{}.Promote()
	if err == nil {
		t.Fatal("Promote() should fail when CI has not passed")
	}
	if !strings.Contains(err.Error(), "CI has not passed") {
		t.Errorf("error = %v, want 'CI has not passed'", err)
	}
}

func TestPromoteCIStale(t *testing.T) {
	m := setupMock(t, "git", "gh")
	m.Return(buffer("next"),
		"git", "branch", "--show-current")
	m.Return(buffer("origin"),
		"git", "config", "--get", "branch.next.remote")
	m.Return(buffer("abc123"),
		"git", "rev-parse", "origin/main")
	m.Return(buffer("def456"),
		"git", "rev-parse", "next")
	m.Return(buffer("old789"), "gh", "run", "list",
		"--branch", "next", "--limit", "1",
		"--json", "headSha", "--jq", ".[0].headSha")

	err := Ops{}.Promote()
	if err == nil {
		t.Fatal("Promote() should fail when CI ran on wrong commit")
	}
	if !strings.Contains(err.Error(), "latest CI run is for") {
		t.Errorf("error = %v, want 'latest CI run is for'", err)
	}
}

func TestPromoteWrongBranch(t *testing.T) {
	m := setupMock(t, "git")
	m.Return(buffer("main"),
		"git", "branch", "--show-current")

	err := Ops{}.Promote()
	if err == nil {
		t.Fatal("Promote() should fail when not on next branch")
	}
	if !strings.Contains(err.Error(), "must be on next branch") {
		t.Errorf("error = %v, want 'must be on next branch'", err)
	}
}

func TestPromoteMainDiverged(t *testing.T) {
	m := setupMock(t, "git")
	m.Return(buffer("next"),
		"git", "branch", "--show-current")
	m.Return(buffer("origin"),
		"git", "config", "--get", "branch.next.remote")
	m.Return(command.Fail(&command.Error{Code: 1}),
		"git", "merge-base", "--is-ancestor",
		"origin/main", "next")

	err := Ops{}.Promote()
	if err == nil {
		t.Fatal("Promote() should fail when main has diverged")
	}
	if !strings.Contains(err.Error(), "cannot fast-forward") {
		t.Errorf("error = %v, want 'cannot fast-forward'", err)
	}
}

func TestPromoteAlreadyEqual(t *testing.T) {
	m := setupMock(t, "git")
	m.Return(buffer("next"),
		"git", "branch", "--show-current")
	m.Return(buffer("origin"),
		"git", "config", "--get", "branch.next.remote")
	m.Return(buffer("abc123"),
		"git", "rev-parse", "origin/main")
	m.Return(buffer("abc123"),
		"git", "rev-parse", "next")

	err := Ops{}.Promote()
	if err == nil {
		t.Fatal("Promote() should fail when next and main are equal")
	}
	if !strings.Contains(err.Error(), "same commit") {
		t.Errorf("error = %v, want 'same commit'", err)
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
