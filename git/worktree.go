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

func MustCopyWorktree(dst, src *cmdio.Runner) {
	if err := CopyWorktree(dst, src); err != nil {
		panic(err)
	}
}

func WorktreeRunner() *cmdio.Runner {
	dir := Runner.MustGet("mktemp", "-d").Out
	ops.Defer(func() { _ = Runner.Run("rm", "-rf", dir) })
	rnr := Runner.WithEnv(map[string]string{"PWD": dir})
	MustCopyWorktree(rnr, Runner)
	return rnr
}
