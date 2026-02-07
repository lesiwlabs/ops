package golib

import (
	"context"
	"sync"
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

func TestCheckRunsOnce(t *testing.T) {
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
	swap(t, &golang.Builder, sh)
	swap(t, &golang.Source, sh)
	swap(t, &golang.InCleanTree,
		func(fn func() error) error { return fn() })
	swap(t, &checkOnce, sync.Once{})
	swap(t, &errCheck, error(nil))

	for range 3 {
		err = Ops{}.Check()
		if err != nil {
			t.Fatal(err)
		}
	}

	got := mock.Calls(m, "go")
	var testCount int
	for _, c := range got {
		if len(c.Args) >= 2 && c.Args[1] == "test" {
			testCount++
		}
	}
	if testCount > 10 {
		t.Errorf("too many test calls (%d), Check should run once",
			testCount)
	}
}
