package golib

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/fs/path"
)

type Target struct {
	Goos   string
	Goarch string
}

var Targets = []Target{
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "386"},
	{"windows", "arm"},
	{"windows", "amd64"},
	{"plan9", "386"},
	{"plan9", "arm"},
	{"plan9", "amd64"},
}

type Ops struct{ golang.Ops }

var checkOnce sync.Once
var errCheck error

func (op Ops) Check() error {
	checkOnce.Do(func() {
		errCheck = golang.Check(op.compile)
	})
	return errCheck
}

func (op Ops) Build() error {
	return op.Check()
}

func (o Ops) Compile() error { return o.compile(context.Background()) }

func (Ops) compile(ctx context.Context) error {
	for _, t := range Targets {
		ctx := command.WithEnv(ctx, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		})
		err := golang.Builder.Exec(ctx,
			"go", "build",
			"-o", golang.DevNull(golang.Builder.OS(ctx)),
			"./...",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (op Ops) Bump() error {
	if err := op.Check(); err != nil {
		return err
	}
	ctx := context.Background()
	_, err := golang.Source.Read(ctx, "which", "bump")
	if err != nil {
		err = golang.Builder.Exec(ctx,
			"go", "install", "lesiw.io/bump@latest")
		if err != nil {
			return err
		}
	}
	which, err := golang.Source.Read(ctx, "which", "bump")
	if err != nil {
		return err
	}
	m := sub.Machine(golang.Source.Unshell(), path.Dir(which))
	bumpsh := command.Shell(m, "bump")

	var versionBuf strings.Builder
	_, err = command.Copy(
		&versionBuf,
		command.NewReader(ctx, golang.Source,
			"git", "describe", "--abbrev=0", "--tags"),
		command.NewStream(ctx, bumpsh, "bump", "-s", "1"),
	)
	if err != nil {
		return err
	}
	version := strings.TrimSpace(versionBuf.String())

	if err := golang.Source.Exec(ctx, "git", "tag", version); err != nil {
		return err
	}
	if err := golang.Source.Exec(ctx, "git", "push"); err != nil {
		return err
	}
	return golang.Source.Exec(ctx, "git", "push", "--tags")
}

func (Ops) ProxyPing() error {
	ctx := context.Background()
	var ref string
	tag, err := golang.Source.Read(ctx,
		"git", "describe", "--exact-match", "--tags")
	if err == nil {
		ref = tag
	} else {
		ref, err = golang.Source.Read(ctx, "git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
	}
	mod, err := golang.Builder.Read(ctx, "go", "list", "-m")
	if err != nil {
		return err
	}
	return golang.Builder.Exec(ctx, "go", "list", "-m",
		fmt.Sprintf("%s@%s", mod, ref))
}
