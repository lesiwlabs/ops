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

func WorktreeRunner(rnr *cmdio.Runner) (*cmdio.Runner, error) {
	dir := rnr.MustGet("mktemp", "-d").Out
	ops.Defer(func() { _ = rnr.Run("rm", "-rf", dir) })
	wtrnr := rnr.WithEnv(map[string]string{"PWD": dir})
	if err := CopyWorktree(wtrnr, rnr); err != nil {
		return nil, err
	}
	return wtrnr, nil
}
