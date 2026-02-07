package golang

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	errname "github.com/Antonboom/errname/pkg/analyzer"
	"golang.org/x/tools/go/analysis"
	gochecker "golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/gofix"
	"golang.org/x/tools/go/analysis/passes/hostport"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/waitgroup"
	"golang.org/x/tools/go/packages"

	"lesiw.io/checker"
	"lesiw.io/command"
	"lesiw.io/command/sys"
	"lesiw.io/errcheck/errcheck"
	"lesiw.io/fs/path"
	"lesiw.io/linelen"
	"lesiw.io/plscheck/deprecated"
	"lesiw.io/plscheck/embeddirective"
	"lesiw.io/plscheck/fillreturns"
	"lesiw.io/plscheck/infertypeargs"
	"lesiw.io/plscheck/maprange"
	"lesiw.io/plscheck/modernize"
	"lesiw.io/plscheck/nonewvars"
	"lesiw.io/plscheck/noresultvalues"
	"lesiw.io/plscheck/recursiveiter"
	"lesiw.io/plscheck/simplifycompositelit"
	"lesiw.io/plscheck/simplifyrange"
	"lesiw.io/plscheck/simplifyslice"
	"lesiw.io/plscheck/unusedfunc"
	"lesiw.io/plscheck/unusedparams"
	"lesiw.io/plscheck/unusedvariable"
	"lesiw.io/plscheck/yield"
	"lesiw.io/tidytypes"
)

type Ops struct{}

var (
	Source  = command.Shell(sys.Machine(), "git")
	Builder = command.Shell(sys.Machine(), "go")
)

var GoModReplaceAllowed bool

func (o Ops) Vet() error {
	ctx := context.Background()
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}

	// go mod tidy (all modules)
	for _, mod := range mods {
		if err := Builder.Exec(ctx,
			"go", "-C", mod, "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy in %s: %w", mod, err)
		}
	}
	if err := diffCheck(ctx, "go mod tidy"); err != nil {
		return err
	}

	// goimports (runs on all files from root)
	if err := installGoimports(ctx); err != nil {
		return err
	}
	if err := Builder.Exec(ctx, "goimports",
		"-w", "-local", "lesiw.io,labs.lesiw.io", "."); err != nil {
		return fmt.Errorf("goimports: %w", err)
	}
	if err := diffCheck(ctx, "goimports"); err != nil {
		return err
	}

	// go fix (all modules)
	for _, mod := range mods {
		if err := Builder.Exec(ctx,
			"go", "-C", mod, "fix", "./..."); err != nil {
			return fmt.Errorf("go fix in %s: %w", mod, err)
		}
	}
	if err := diffCheck(ctx, "go fix"); err != nil {
		return err
	}

	// go vet (all modules)
	for _, mod := range mods {
		if err := Builder.Exec(ctx,
			"go", "-C", mod, "vet", "./..."); err != nil {
			return fmt.Errorf("go vet in %s: %w", mod, err)
		}
	}

	// go.mod replace check (already recursive)
	if !GoModReplaceAllowed {
		if err := checkGoModReplace(ctx, Builder); err != nil {
			return err
		}
	}

	// analyzers (all modules)
	if err := runAnalyzers(mods); err != nil {
		return err
	}

	// short tests (all modules, twice: without and with race detector)
	if err := runTests(ctx, mods, true); err != nil {
		return err
	}
	return nil
}

func (Ops) Test() error {
	ctx := context.Background()
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}
	return runTests(ctx, mods, false)
}

func (o Ops) Fix() error {
	ctx := context.Background()
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}

	// go mod tidy (all modules)
	for _, mod := range mods {
		if err := Builder.Exec(ctx,
			"go", "-C", mod, "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy in %s: %w", mod, err)
		}
	}

	// goimports (runs on all files from root)
	if err := installGoimports(ctx); err != nil {
		return err
	}
	if err := Builder.Exec(ctx, "goimports",
		"-w", "-local", "lesiw.io,labs.lesiw.io", "."); err != nil {
		return fmt.Errorf("goimports: %w", err)
	}

	// go fix (all modules)
	for _, mod := range mods {
		if err := Builder.Exec(ctx,
			"go", "-C", mod, "fix", "./..."); err != nil {
			return fmt.Errorf("go fix in %s: %w", mod, err)
		}
	}

	// fixable analyzers (all modules)
	return runFixAnalyzers(mods)
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

	if err := Builder.Exec(ctx, "go", "test",
		"-coverprofile", coverOut.Path(), "./..."); err != nil {
		return err
	}
	return Builder.Exec(ctx, "go", "tool", "cover",
		"-html="+coverOut.Path())
}

func modules(ctx context.Context) ([]string, error) {
	var mods []string
	if err := findModules(ctx, Builder, ".", &mods); err != nil {
		return nil, err
	}
	return mods, nil
}

func findModules(
	ctx context.Context,
	sh *command.Sh,
	dir string,
	mods *[]string,
) error {
	for entry, err := range sh.ReadDir(ctx, dir) {
		if err != nil {
			return fmt.Errorf("read directory %s: %w", dir, err)
		}
		name := entry.Name()
		if name == ".git" || name == "vendor" {
			continue
		}
		entryPath := path.Join(dir, name)
		if entry.IsDir() {
			err := findModules(ctx, sh, entryPath, mods)
			if err != nil {
				return err
			}
			continue
		}
		if name == "go.mod" {
			*mods = append(*mods, dir)
		}
	}
	return nil
}

func runTests(ctx context.Context, mods []string, short bool) error {
	shortFlag := []string{}
	if short {
		shortFlag = []string{"-short"}
	}

	for _, mod := range mods {
		// Pass 1: CGO_ENABLED=0, no race detector
		args := append(
			[]string{"go", "-C", mod,
				"test", "-count=1", "-shuffle=on"},
			shortFlag...)
		args = append(args, "./...")
		ctx0 := command.WithEnv(ctx,
			map[string]string{"CGO_ENABLED": "0"})
		if err := Builder.Exec(ctx0, args...); err != nil {
			return fmt.Errorf("test (no race) in %s: %w", mod, err)
		}

		// Pass 2: CGO_ENABLED=1, race detector
		args = append(
			[]string{"go", "-C", mod,
				"test", "-count=1", "-shuffle=on", "-race"},
			shortFlag...)
		args = append(args, "./...")
		ctx1 := command.WithEnv(ctx,
			map[string]string{"CGO_ENABLED": "1"})
		if err := Builder.Exec(ctx1, args...); err != nil {
			return fmt.Errorf("test (race) in %s: %w", mod, err)
		}
	}
	return nil
}

func diffCheck(ctx context.Context, step string) error {
	diff, err := Source.Read(ctx, "git", "diff", "--name-only")
	if err != nil {
		return fmt.Errorf("git diff after %s: %w", step, err)
	}
	if diff != "" {
		return fmt.Errorf("%s produced changes:\n%s", step, diff)
	}
	return nil
}

func installGoimports(ctx context.Context) error {
	err := command.Do(ctx, Builder.Unshell(), "goimports", "-l")
	if command.NotFound(err) {
		return Builder.Exec(ctx,
			"go", "install",
			"golang.org/x/tools/cmd/goimports@latest")
	}
	return nil
}

func runAnalyzers(mods []string) error {
	for _, mod := range mods {
		pkgs, err := packages.Load(&packages.Config{
			Dir:   mod,
			Mode:  packages.LoadAllSyntax,
			Tests: true,
		}, "./...")
		if err != nil {
			return fmt.Errorf("load packages in %s: %w", mod, err)
		}
		graph, err := gochecker.Analyze(
			[]*analysis.Analyzer{
				checker.NewAnalyzer(Analyzers()...),
			},
			pkgs, nil,
		)
		if err != nil {
			return fmt.Errorf("run analyzers in %s: %w", mod, err)
		}
		var buf bytes.Buffer
		if err := graph.PrintText(&buf, 0); err != nil {
			return fmt.Errorf("print diagnostics: %w", err)
		}
		if buf.Len() > 0 {
			return fmt.Errorf(
				"analyzers found issues in %s:\n%s",
				mod, buf.String())
		}
	}
	return nil
}

func runFixAnalyzers(mods []string) error {
	for _, mod := range mods {
		pkgs, err := packages.Load(&packages.Config{
			Dir:   mod,
			Mode:  packages.LoadAllSyntax,
			Tests: true,
		}, "./...")
		if err != nil {
			return fmt.Errorf("load packages in %s: %w", mod, err)
		}
		if len(pkgs) == 0 {
			continue
		}
		graph, err := gochecker.Analyze(
			[]*analysis.Analyzer{
				checker.NewAnalyzer(FixAnalyzers()...),
			},
			pkgs, nil,
		)
		if err != nil {
			return fmt.Errorf(
				"run fix analyzers in %s: %w", mod, err)
		}
		if err := applyFixes(pkgs, graph); err != nil {
			return fmt.Errorf("apply fixes in %s: %w", mod, err)
		}
	}
	return nil
}

func applyFixes(
	pkgs []*packages.Package,
	graph *gochecker.Graph,
) error {
	if len(pkgs) == 0 {
		return nil
	}
	fset := pkgs[0].Fset

	type edit struct {
		start, end int
		newText    []byte
	}
	fileEdits := make(map[string][]edit)

	for act := range graph.All() {
		for _, d := range act.Diagnostics {
			for _, fix := range d.SuggestedFixes {
				for _, te := range fix.TextEdits {
					pos := fset.Position(te.Pos)
					end := fset.Position(te.End)
					fileEdits[pos.Filename] = append(
						fileEdits[pos.Filename], edit{
							start:   pos.Offset,
							end:     end.Offset,
							newText: te.NewText,
						})
				}
			}
		}
	}

	for filename, edits := range fileEdits {
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read %s: %w", filename, err)
		}
		slices.SortFunc(edits, func(a, b edit) int {
			return b.start - a.start
		})
		for _, e := range edits {
			content = slices.Replace(
				content, e.start, e.end, e.newText...)
		}
		if err := os.WriteFile(filename, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return nil
}

// Analyzers returns the standard set of analyzers.
func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		atomicalign.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		deprecated.Analyzer,
		embeddirective.Analyzer,
		errcheck.Analyzer,
		errname.New(),
		fillreturns.Analyzer,
		gofix.Analyzer,
		hostport.Analyzer,
		httpmux.Analyzer,
		infertypeargs.Analyzer,
		linelen.Analyzer,
		maprange.Analyzer,
		modernize.Analyzer,
		nilness.Analyzer,
		nonewvars.Analyzer,
		noresultvalues.Analyzer,
		recursiveiter.Analyzer,
		reflectvaluecompare.Analyzer,
		simplifycompositelit.Analyzer,
		simplifyrange.Analyzer,
		simplifyslice.Analyzer,
		sortslice.Analyzer,
		tidytypes.Analyzer,
		unusedfunc.Analyzer,
		unusedparams.Analyzer,
		unusedvariable.Analyzer,
		unusedwrite.Analyzer,
		waitgroup.Analyzer,
		yield.Analyzer,
	}
}

// FixAnalyzers returns analyzers that provide suggested fixes.
func FixAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		fillreturns.Analyzer,
		infertypeargs.Analyzer,
		modernize.Analyzer,
		simplifycompositelit.Analyzer,
		simplifyrange.Analyzer,
		simplifyslice.Analyzer,
	}
}

func checkGoModReplace(ctx context.Context, sh *command.Sh) error {
	var foundReplace []string
	err := checkGoModReplaceDir(ctx, sh, ".", &foundReplace)
	if err != nil {
		return err
	}
	if len(foundReplace) > 0 {
		return fmt.Errorf(
			"replace directive found in go.mod\n%s",
			strings.Join(foundReplace, "\n"))
	}
	return nil
}

func checkGoModReplaceDir(
	ctx context.Context,
	sh *command.Sh,
	dir string,
	foundReplace *[]string,
) error {
	for entry, err := range sh.ReadDir(ctx, dir) {
		if err != nil {
			return fmt.Errorf(
				"failed to read directory %s: %w", dir, err)
		}
		entryPath := path.Join(dir, entry.Name())
		if entry.IsDir() {
			err := checkGoModReplaceDir(
				ctx, sh, entryPath, foundReplace)
			if err != nil {
				return err
			}
			continue
		}
		if entry.Name() != "go.mod" {
			continue
		}
		f, err := sh.Open(ctx, entryPath)
		if err != nil {
			return fmt.Errorf(
				"failed to open %s: %w", entryPath, err)
		}
		defer f.Close()
		scn := bufio.NewScanner(f)
		lineNum := 0
		for scn.Scan() {
			lineNum++
			line := scn.Text()
			if strings.HasPrefix(
				strings.TrimSpace(line), "replace") {
				*foundReplace = append(*foundReplace,
					fmt.Sprintf("%s:%d:%s",
						entryPath, lineNum, line))
			}
		}
		if err := scn.Err(); err != nil {
			return fmt.Errorf(
				"failed to scan %s: %w", entryPath, err)
		}
	}
	return nil
}
