package golang

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"lesiw.io/command"
	"lesiw.io/command/sys"
	"lesiw.io/fs/path"
)

type Ops struct{}

var (
	Source  = command.Shell(sys.Machine(), "git")
	Builder = command.Shell(sys.Machine(), "go")
)

var GoModReplaceAllowed bool

func (Ops) Test() error {
	ctx := context.Background()
	return GoTestSum.Exec(ctx, "gotestsum", "./...", "--", "-race")
}

func (Ops) Lint() error {
	ctx := context.Background()
	if !GoModReplaceAllowed {
		return checkGoModReplace(ctx, Builder)
	}
	return nil
}

func (Ops) Cov() error {
	ctx := context.Background()
	tmpDir, err := Builder.Temp(ctx, "gocover/")
	if err != nil {
		return err
	}
	defer tmpDir.Close()
	defer Builder.RemoveAll(ctx, tmpDir.Path())

	coverOutPath := path.Join(tmpDir.Path(), "cover.out")
	coverOut, err := Builder.Create(ctx, coverOutPath)
	if err != nil {
		return err
	}
	defer coverOut.Close()

	if err := Builder.Exec(ctx, "go", "test", "-coverprofile", coverOut.Path(), "./..."); err != nil {
		return err
	}
	return Builder.Exec(ctx, "go", "tool", "cover", "-html="+coverOut.Path())
}

func checkGoModReplace(ctx context.Context, sh *command.Sh) error {
	var foundReplace []string
	err := checkGoModReplaceDir(ctx, sh, ".", &foundReplace)
	if err != nil {
		return err
	}
	if len(foundReplace) > 0 {
		return fmt.Errorf("replace directive found in go.mod\n%s", strings.Join(foundReplace, "\n"))
	}
	return nil
}

func checkGoModReplaceDir(ctx context.Context, sh *command.Sh, dir string, foundReplace *[]string) error {
	for entry, err := range sh.ReadDir(ctx, dir) {
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", dir, err)
		}
		entryPath := path.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := checkGoModReplaceDir(ctx, sh, entryPath, foundReplace); err != nil {
				return err
			}
			continue
		}
		if entry.Name() != "go.mod" {
			continue
		}
		f, err := sh.Open(ctx, entryPath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", entryPath, err)
		}
		defer f.Close()
		scn := bufio.NewScanner(f)
		lineNum := 0
		for scn.Scan() {
			lineNum++
			line := scn.Text()
			if strings.HasPrefix(strings.TrimSpace(line), "replace") {
				*foundReplace = append(*foundReplace, fmt.Sprintf("%s:%d:%s", entryPath, lineNum, line))
			}
		}
		if err := scn.Err(); err != nil {
			return fmt.Errorf("failed to scan %s: %w", entryPath, err)
		}
	}
	return nil
}
