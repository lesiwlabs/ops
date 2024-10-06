package git

import (
	"lesiw.io/cmdio"
	"lesiw.io/ops"
)

func CopyWorktree(dst, src *cmdio.Runner) error {
	return cmdio.Pipe(
		src.Command("git", "archive", "HEAD"),
		dst.Command("tar", "xf", "-"),
	)
}

func WorktreeRunner() (*cmdio.Runner, error) {
	dir := Runner.MustGet("mktemp", "-d").Out
	ops.Defer(func() { _ = Runner.Run("rm", "-rf", dir) })
	rnr := Runner.WithEnv(map[string]string{"PWD": dir})
	if err := CopyWorktree(rnr, Runner); err != nil {
		return nil, err
	}
	return rnr, nil
}
