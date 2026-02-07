package git

import (
	"context"
	"fmt"
	"io"

	"lesiw.io/command"
)

func CopyWorktree(ctx context.Context, dstDir string, src *command.Sh) error {
	gitArchive := command.NewReader(ctx, src, "git", "archive", "HEAD")
	tar, err := src.Create(ctx, dstDir)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	defer tar.Close()
	_, err = io.Copy(tar, gitArchive)
	if err != nil {
		return fmt.Errorf("failed to copy worktree: %w", err)
	}
	return nil
}

func WorktreeShell(ctx context.Context, sh *command.Sh) (*command.Sh, error) {
	return sh, nil
}
