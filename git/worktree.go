package git

import (
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
	"lesiw.io/cmdio/x/busybox"
	"lesiw.io/ops"
)

func CopyWorktree(dst, src *cmdio.Runner) error {
	return cmdio.Pipe(
		src.Command("git", "archive", "HEAD"),
		dst.Command("tar", "xf", "-"),
	)
}

func WorktreeRunner() (*cmdio.Runner, error) {
	rnr, err := busybox.Runner()
	if err != nil {
		return nil, err
	}
	dir := rnr.MustGet("mktemp", "-d").Out
	ops.Defer(func() { _ = rnr.Run("rm", "-rf", dir) })
	rnr = rnr.WithEnv(map[string]string{"PWD": dir})
	if err := CopyWorktree(rnr, sys.Runner()); err != nil {
		return nil, err
	}
	return sys.Runner().WithEnv(map[string]string{"PWD": dir}), nil
}
