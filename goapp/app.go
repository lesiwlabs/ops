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
		if err := golang.Builder.Exec(ctx,
			"go", "build", "-ldflags=-s -w", "-o",
			"out/"+Name+"-"+t.Unames+"-"+t.Unamer, ".",
		); err != nil {
			return err
		}
	}
	builderOut, err := golang.Builder.Open(ctx, "out/")
	if err != nil {
		return err
	}
	defer builderOut.Close()
	sourceOut, err := golang.Source.Create(ctx, "out/")
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
	if err := golang.Source.RemoveAll(ctx, "out"); err != nil {
		return err
	}
	return golang.Source.MkdirAll(ctx, "out")
}

func (Ops) Bump() error {
	ctx := context.Background()
	_, err := golang.Source.Read(ctx, "which", "bump")
	if err != nil {
		if err := golang.Builder.Exec(ctx, "go", "install", "lesiw.io/bump@latest"); err != nil {
			return err
		}
	}
	curVersion, err := golang.Builder.ReadFile(ctx, Versionfile)
	if err != nil {
		return err
	}
	which, err := golang.Source.Read(ctx, "which", "bump")
	if err != nil {
		return err
	}
	bumpsh := command.Shell(sub.Machine(golang.Source.Unshell(), path.Dir(which)), "bump")

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

	if err := golang.Source.WriteFile(ctx, Versionfile, []byte(version+"\n")); err != nil {
		return err
	}
	if err := golang.Source.Exec(ctx, "git", "add", Versionfile); err != nil {
		return err
	}
	if err := golang.Source.Exec(ctx, "git", "commit", "-m", version); err != nil {
		return err
	}
	if err := golang.Source.Exec(ctx, "git", "tag", version); err != nil {
		return err
	}
	if err := golang.Source.Exec(ctx, "git", "push"); err != nil {
		return err
	}
	return golang.Source.Exec(ctx, "git", "push", "--tags")
}
