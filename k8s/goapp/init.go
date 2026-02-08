//go:build !test

package goapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"lesiw.io/command"
	"lesiw.io/command/sub"
	"lesiw.io/command/sys"
	"lesiw.io/defers"
)

func init() {
	if err := runinit(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		defers.Exit(1)
	}
}

func runinit() (err error) {
	ctx := context.Background()
	m := sys.Machine()
	sh := command.Shell(m, "go")

	if err := command.Do(ctx, m, "spkez", "--help"); command.NotFound(err) {
		err := sh.Exec(ctx,
			"go", "install", "lesiw.io/spkez@latest",
		)
		if err != nil {
			return fmt.Errorf("could not install spkez: %w", err)
		}
	}
	spkez = sub.Machine(m, "spkez")

	config, err := command.Read(ctx, spkez, "get", "k8s/config")
	if err != nil {
		return fmt.Errorf("could not get kubeconfig: %w", err)
	}
	file, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	defers.Add(func() { _ = os.Remove(file.Name()) })
	defer file.Close()
	if err := os.Chmod(file.Name(), 0600); err != nil {
		return fmt.Errorf("could not set permissions on temp file: %w", err)
	}
	if _, err := file.WriteString(config + "\n"); err != nil {
		return fmt.Errorf("could not write to temp file: %w", err)
	}
	k8scfg, err = filepath.Abs(file.Name())
	if err != nil {
		return fmt.Errorf("could not get full path of temp file: %w", err)
	}

	ctr = sub.Machine(m, "docker")
	k8s = sub.Machine(ctr, "run", "-i", "--rm",
		"-v", k8scfg+":/.kube/config",
		"bitnami/kubectl",
	)
	return nil
}
