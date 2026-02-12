package goapp

import (
	"context"
	"fmt"
	"io"
	"strings"

	"labs.lesiw.io/ops/golang"
	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/fs/path"
)

var Targets = []golang.Target{
	{Goos: "linux", Goarch: "386"},
	{Goos: "linux", Goarch: "amd64"},
	{Goos: "linux", Goarch: "arm"},
	{Goos: "linux", Goarch: "arm64"},
	{Goos: "darwin", Goarch: "amd64"},
	{Goos: "darwin", Goarch: "arm64"},
}

type Ops struct{ golang.Ops }

var Name string
var Versionfile = "version.txt"

func (op Ops) Build(ctx context.Context) error {
	if Name == "" {
		return fmt.Errorf("no app name given")
	}
	if err := op.Clean(ctx); err != nil {
		return err
	}
	if err := op.Lint(ctx); err != nil {
		return err
	}
	if err := op.Test(ctx); err != nil {
		return err
	}
	for _, t := range Targets {
		ctx := command.WithEnv(ctx, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		})
		if err := golang.Build.Exec(ctx,
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames()+"-"+t.Unamer(), ".",
		); err != nil {
			return err
		}
	}
	if golang.Build.Unshell() == golang.Local.Unshell() {
		return nil
	}
	builderOut, err := golang.Build.Open(ctx, "out/")
	if err != nil {
		return err
	}
	defer builderOut.Close()
	sourceOut, err := golang.Local.Create(ctx, "out/")
	if err != nil {
		return err
	}
	defer sourceOut.Close()
	if _, err := io.Copy(sourceOut, builderOut); err != nil {
		return err
	}
	return nil
}

func (Ops) Clean(ctx context.Context) error {
	if err := golang.Local.RemoveAll(ctx, "out"); err != nil {
		return err
	}
	return golang.Local.MkdirAll(ctx, "out")
}

func (Ops) Bump(ctx context.Context) error {
	_, err := golang.Local.Read(ctx, "which", "bump")
	if err != nil {
		err = golang.Build.Exec(ctx,
			"go", "install", "lesiw.io/bump@latest")
		if err != nil {
			return err
		}
	}
	curVersion, err := golang.Build.ReadFile(ctx, Versionfile)
	if err != nil {
		return err
	}
	which, err := golang.Local.Read(ctx, "which", "bump")
	if err != nil {
		return err
	}
	m := sub.Machine(
		golang.Local.Unshell(), path.Dir(which))
	bumpsh := command.Shell(m, "bump")

	var versionBuf strings.Builder
	_, err = command.Copy(
		&versionBuf,
		strings.NewReader(string(curVersion)+"\n"),
		command.NewStream(ctx, bumpsh, "bump", "-s", "1"),
	)
	if err != nil {
		return err
	}
	version := strings.TrimSpace(versionBuf.String())

	err = golang.Local.WriteFile(ctx,
		Versionfile, []byte(version+"\n"))
	if err != nil {
		return err
	}
	err = golang.Local.Exec(ctx, "git", "add", Versionfile)
	if err != nil {
		return err
	}
	err = golang.Local.Exec(ctx,
		"git", "commit", "-m", version)
	if err != nil {
		return err
	}
	if err := golang.Local.Exec(ctx, "git", "tag", version); err != nil {
		return err
	}
	if err := golang.Local.Exec(ctx, "git", "push"); err != nil {
		return err
	}
	return golang.Local.Exec(ctx, "git", "push", "--tags")
}
