//go:build !test
// +build !test

package goapp

import (
	"fmt"
	"os"
	"path/filepath"

	"lesiw.io/cmdio/sub"
	"lesiw.io/cmdio/x/busybox"
	"lesiw.io/defers"
)

func init() {
	if err := runinit(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		defers.Exit(1)
	}
}

func runinit() (err error) {
	rnr, err = busybox.Runner()
	if err != nil {
		return fmt.Errorf("could not create busybox runner: %w", err)
	}

	r, err := rnr.Get("which", "spkez")
	if err != nil {
		err := rnr.Run("go", "install", "lesiw.io/spkez@latest")
		if err != nil {
			return fmt.Errorf("could not install spkez: %w", err)
		}
		r, err = rnr.Get("which", "spkez")
		if err != nil {
			return fmt.Errorf("could not find spkez: %w", err)
		}
	}
	spkez = sub.WithRunner(rnr, r.Out)

	r, err = spkez.Get("get", "k8s/config")
	if err != nil {
		return fmt.Errorf("could not get kubeconfig: %w", err)
	}
	file, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	defers.Add(func() { os.Remove(file.Name()) })
	defer file.Close()
	if err := os.Chmod(file.Name(), 0600); err != nil {
		return fmt.Errorf("could not set permissions on temp file: %w", err)
	}
	if _, err := file.WriteString(r.Out + "\n"); err != nil {
		return fmt.Errorf("could not write to temp file: %w", err)
	}
	k8scfg, err = filepath.Abs(file.Name())
	if err != nil {
		return fmt.Errorf("could not get full path of temp file: %w", err)
	}

	ctr = sub.WithRunner(rnr, "docker")
	k8s = sub.WithRunner(ctr,
		"run", "-i", "--rm",
		"-v", k8scfg+":/.kube/config",
		"bitnami/kubectl",
	)

	return nil
}
