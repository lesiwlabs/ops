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

type Target struct {
	Goos   string
	Goarch string
	Unames string
	Unamer string
}

var Targets = []Target{
	{"linux", "386", "linux", "i386"},
	{"linux", "amd64", "linux", "x86_64"},
	{"linux", "arm", "linux", "armv7l"},
	{"linux", "arm64", "linux", "aarch64"},
	{"darwin", "amd64", "darwin", "x86_64"},
	{"darwin", "arm64", "darwin", "arm64"},
}

type Ops struct{ golang.Ops }

var Name string
var Versionfile = "version.txt"

func (op Ops) Check() error {
	return golang.Check(op.compile)
}

func (o Ops) Compile() error { return o.compile(context.Background()) }

func (Ops) compile(ctx context.Context) error {
	for _, t := range Targets {
		ctx := command.WithEnv(ctx, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		})
		err := golang.Build.Exec(ctx,
			"go", "build",
			"-o", golang.DevNull(golang.Build.OS(ctx)),
			".",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (op Ops) Build() error {
	if Name == "" {
		return fmt.Errorf("no app name given")
	}
	if err := op.Clean(); err != nil {
		return err
	}
	if err := op.Lint(); err != nil {
		return err
	}
	if err := op.Test(); err != nil {
		return err
	}
	ctx := context.Background()
	for _, t := range Targets {
		ctx := command.WithEnv(ctx, map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        t.Goos,
			"GOARCH":      t.Goarch,
		})
		if err := golang.Build.Exec(ctx,
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames+"-"+t.Unamer, ".",
		); err != nil {
			return err
		}
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

func (Ops) Clean() error {
	ctx := context.Background()
	if err := golang.Local.RemoveAll(ctx, "out"); err != nil {
		return err
	}
	return golang.Local.MkdirAll(ctx, "out")
}

func (Ops) Bump() error {
	ctx := context.Background()
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
