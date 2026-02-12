package golib

import (
	"context"
	"fmt"
	"strings"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/fs/path"
)

type Ops struct{ golang.Ops }

func (op Ops) Build(ctx context.Context) error {
	return op.Check(ctx)
}

func (op Ops) Bump(ctx context.Context) error {
	if err := op.Check(ctx); err != nil {
		return err
	}
	_, err := golang.Local.Read(ctx, "which", "bump")
	if err != nil {
		err = golang.Build.Exec(ctx,
			"go", "install", "lesiw.io/bump@latest")
		if err != nil {
			return err
		}
	}
	which, err := golang.Local.Read(ctx, "which", "bump")
	if err != nil {
		return err
	}
	m := sub.Machine(golang.Local.Unshell(), path.Dir(which))
	bumpsh := command.Shell(m, "bump")

	var versionBuf strings.Builder
	_, err = command.Copy(
		&versionBuf,
		command.NewReader(ctx, golang.Local,
			"git", "describe", "--abbrev=0", "--tags"),
		command.NewStream(ctx, bumpsh, "bump", "-s", "1"),
	)
	if err != nil {
		return err
	}
	version := strings.TrimSpace(versionBuf.String())

	if err := golang.Local.Exec(ctx, "git", "tag", version); err != nil {
		return err
	}
	if err := golang.Local.Exec(ctx, "git", "push"); err != nil {
		return err
	}
	return golang.Local.Exec(ctx, "git", "push", "--tags")
}

func (Ops) ProxyPing(ctx context.Context) error {
	var ref string
	tag, err := golang.Local.Read(ctx,
		"git", "describe", "--exact-match", "--tags")
	if err == nil {
		ref = tag
	} else {
		ref, err = golang.Local.Read(ctx, "git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
	}
	mod, err := golang.Build.Read(ctx, "go", "list", "-m")
	if err != nil {
		return err
	}
	return golang.Build.Exec(ctx, "go", "list", "-m",
		fmt.Sprintf("%s@%s", mod, ref))
}
