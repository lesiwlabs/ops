package golib

import (
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
	m := new(mock.Machine)
	m.SetOS("linux")
	m.SetArch("amd64")
	sh := command.Shell(m, "go", "gotestsum", "git")
	swap(t, &golang.Builder, sh)
	swap(t, &golang.Source, sh)
	swap(t, &golang.GoTestSum, sh)
	swap(t, &checkOnce, sync.Once{})
	swap(t, &checkErr, error(nil))

	for range 3 {
		Ops{}.Check()
	}

	got := mock.Calls(m, "gotestsum")
	if len(got) != 1 {
		t.Errorf("gotestsum runs: %d, want 1", len(got))
	}
}
