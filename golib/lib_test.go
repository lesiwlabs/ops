package golib

import (
	"context"
	"testing"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/command"
	"lesiw.io/command/mock"
)

func init() {
	golang.GoModReplaceAllowed = true
}

func swap[T any](t *testing.T, ptr *T, val T) {
	t.Helper()
	old := *ptr
	*ptr = val
	t.Cleanup(func() { *ptr = old })
}

func TestCheckInherited(t *testing.T) {
	ctx := context.Background()
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, "go", "git", "goimports")
	err := sh.WriteFile(ctx,
		"go.mod", []byte("module test\n"))
	if err != nil {
		t.Fatal(err)
	}
	swap(t, &golang.Build, sh)
	swap(t, &golang.Local, sh)
	swap(t, &golang.InCleanTree,
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	err = Ops{}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	got := mock.Calls(m, "go")
	var buildCount int
	for _, c := range got {
		if len(c.Args) >= 2 && c.Args[1] == "build" {
			buildCount++
		}
	}
	if buildCount != len(golang.CheckTargets) {
		t.Errorf("got %d build calls, want %d",
			buildCount, len(golang.CheckTargets))
	}
}
