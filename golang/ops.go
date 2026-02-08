package golang

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	errname "github.com/Antonboom/errname/pkg/analyzer"
	"github.com/google/go-cmp/cmp"
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
	"lesiw.io/fs"
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
	Local = command.Shell(sys.Machine(), "git")
	Build = command.Shell(sys.Machine(), "go")
)

var GoModReplaceAllowed bool

func (o Ops) Vet() error { return o.vet(context.Background()) }

func (o Ops) vet(ctx context.Context) error {
	// Take initial snapshot if not already set (e.g. direct Vet() call).
	if snapshotFromContext(ctx) == nil {
		snap, err := takeSnapshot(ctx)
		if err != nil {
			return fmt.Errorf("initial snapshot: %w", err)
		}
		ctx = withSnapshot(ctx, snap)
	}
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}

	// go mod tidy (all modules)
	for _, mod := range mods {
		err = Build.Exec(ctx, "go", "-C", mod, "mod", "tidy")
		if err != nil {
			return fmt.Errorf("go mod tidy in %s: %w", mod, err)
		}
	}
	if err = diffCheck(ctx, "go mod tidy"); err != nil {
		return err
	}

	// goimports (runs on all files from root)
	if err = installGoimports(ctx); err != nil {
		return err
	}
	err = Build.Exec(ctx, "goimports",
		"-w", "-local", "lesiw.io,labs.lesiw.io", ".")
	if err != nil {
		return fmt.Errorf("goimports: %w", err)
	}
	if err = diffCheck(ctx, "goimports"); err != nil {
		return err
	}

	// go fix (all modules)
	for _, mod := range mods {
		err = Build.Exec(ctx, "go", "-C", mod, "fix", "./...")
		if err != nil {
			return fmt.Errorf("go fix in %s: %w", mod, err)
		}
	}
	if err = diffCheck(ctx, "go fix"); err != nil {
		return err
	}

	// go vet (all modules)
	for _, mod := range mods {
		err = Build.Exec(ctx, "go", "-C", mod, "vet", "./...")
		if err != nil {
			return fmt.Errorf("go vet in %s: %w", mod, err)
		}
	}

	// go.mod replace check (already recursive)
	if !GoModReplaceAllowed {
		if err := checkGoModReplace(ctx, Build); err != nil {
			return err
		}
	}

	// analyzers (all modules)
	if err := runAnalyzers(ctx, mods); err != nil {
		return err
	}

	// short tests (all modules, twice: without and with race detector)
	if err := runTests(ctx, mods, true); err != nil {
		return err
	}
	return nil
}

func (o Ops) Test() error { return o.test(context.Background()) }

func (Ops) test(ctx context.Context) error {
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}
	return runTests(ctx, mods, false)
}

func (o Ops) Fix() error { return o.fix(context.Background()) }

func (o Ops) fix(ctx context.Context) error {
	mods, err := modules(ctx)
	if err != nil {
		return fmt.Errorf("find modules: %w", err)
	}

	// go mod tidy (all modules)
	for _, mod := range mods {
		err = Build.Exec(ctx, "go", "-C", mod, "mod", "tidy")
		if err != nil {
			return fmt.Errorf("go mod tidy in %s: %w", mod, err)
		}
	}

	// goimports (runs on all files from root)
	if err = installGoimports(ctx); err != nil {
		return err
	}
	err = Build.Exec(ctx, "goimports",
		"-w", "-local", "lesiw.io,labs.lesiw.io", ".")
	if err != nil {
		return fmt.Errorf("goimports: %w", err)
	}

	// go fix (all modules)
	for _, mod := range mods {
		err = Build.Exec(ctx, "go", "-C", mod, "fix", "./...")
		if err != nil {
			return fmt.Errorf("go fix in %s: %w", mod, err)
		}
	}

	// fixable analyzers (all modules)
	return runFixAnalyzers(ctx, mods)
}

func (Ops) Lint() error {
	ctx := context.Background()
	if !GoModReplaceAllowed {
		return checkGoModReplace(ctx, Build)
	}
	return nil
}

func (Ops) Cov() error {
	ctx := context.Background()
	tmpDir, err := Build.Temp(ctx, "gocover/")
	if err != nil {
		return err
	}
	defer tmpDir.Close()
	defer Build.RemoveAll(ctx, tmpDir.Path())

	coverOutPath := path.Join(tmpDir.Path(), "cover.out")
	coverOut, err := Build.Create(ctx, coverOutPath)
	if err != nil {
		return err
	}
	defer coverOut.Close()

	err = Build.Exec(ctx, "go", "test",
		"-coverprofile", coverOut.Path(), "./...")
	if err != nil {
		return err
	}
	return Build.Exec(ctx, "go", "tool", "cover",
		"-html="+coverOut.Path())
}

// Check runs vet, compile, and test in a clean tree.
// The compile parameter is called between vet and test.
// Pass nil to skip compilation.
func Check(compile func(context.Context) error) error {
	return InCleanTree(func(ctx context.Context) error {
		o := Ops{}
		if err := o.vet(ctx); err != nil {
			return err
		}
		if compile != nil {
			if err := compile(ctx); err != nil {
				return err
			}
		}
		return o.test(ctx)
	})
}

// InCleanTree extracts the committed git tree into a
// temporary directory and runs fn there. This ensures checks
// run against committed state only.
var InCleanTree = inCleanTree

func inCleanTree(fn func(context.Context) error) error {
	ctx := context.Background()
	tmp, err := Local.Temp(ctx, "op-check/")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer Local.RemoveAll(ctx, tmp.Path())

	// Extract committed tree into temp dir, capturing snapshot.
	archive := command.NewReader(ctx,
		Local, "git", "archive", "HEAD")
	tee := io.TeeReader(archive, tmp)
	before, err := parseTarSnapshot(tee)
	if err != nil {
		return fmt.Errorf("parse archive: %w", err)
	}
	if _, err = io.Copy(io.Discard, tee); err != nil {
		return fmt.Errorf("drain archive: %w", err)
	}
	if err = archive.Close(); err != nil {
		return fmt.Errorf("archive close: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("temp close: %w", err)
	}

	ctx = fs.WithWorkDir(ctx, tmp.Path())
	ctx = withSnapshot(ctx, before)
	return fn(ctx)
}

func modules(ctx context.Context) ([]string, error) {
	var mods []string
	if err := findModules(ctx, Build, ".", &mods); err != nil {
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
		if err := Build.Exec(ctx0, args...); err != nil {
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
		if err := Build.Exec(ctx1, args...); err != nil {
			return fmt.Errorf("test (race) in %s: %w", mod, err)
		}
	}
	return nil
}

func diffCheck(ctx context.Context, step string) error {
	before := snapshotFromContext(ctx)
	if before == nil {
		return nil
	}
	after, err := takeSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("snapshot after %s: %w", step, err)
	}

	var buf strings.Builder
	for _, name := range slices.Sorted(maps.Keys(before)) {
		afterContent, ok := after[name]
		if !ok {
			fmt.Fprintf(&buf, "deleted: %s\n", name)
			continue
		}
		if before[name] != afterContent {
			diff := cmp.Diff(
				strings.Split(before[name], "\n"),
				strings.Split(afterContent, "\n"),
			)
			fmt.Fprintf(&buf, "--- %s\n%s", name, diff)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[name]; !ok {
			fmt.Fprintf(&buf, "added: %s\n", name)
		}
	}

	if buf.Len() > 0 {
		return fmt.Errorf("%s produced changes:\n%s",
			step, buf.String())
	}
	return nil
}

type snapshotKey struct{}

func withSnapshot(
	ctx context.Context, snap map[string]string,
) context.Context {
	return context.WithValue(ctx, snapshotKey{}, snap)
}

func snapshotFromContext(ctx context.Context) map[string]string {
	snap, _ := ctx.Value(snapshotKey{}).(map[string]string)
	return snap
}

func parseTarSnapshot(r io.Reader) (map[string]string, error) {
	snap := make(map[string]string)
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return snap, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		snap[path.Join(".", hdr.Name)] = strings.ReplaceAll(
			string(data), "\r\n", "\n",
		)
	}
}

func takeSnapshot(
	ctx context.Context,
) (map[string]string, error) {
	snap := make(map[string]string)
	if err := walkSnapshot(ctx, ".", snap); err != nil {
		return nil, err
	}
	return snap, nil
}

func walkSnapshot(
	ctx context.Context, dir string, snap map[string]string,
) error {
	for entry, err := range Build.ReadDir(ctx, dir) {
		if err != nil {
			return err
		}
		name := entry.Name()
		if name == ".git" || name == "vendor" {
			continue
		}
		p := path.Join(dir, name)
		if entry.IsDir() {
			if err := walkSnapshot(ctx, p, snap); err != nil {
				return err
			}
			continue
		}
		data, err := Build.ReadFile(ctx, p)
		if err != nil {
			return err
		}
		snap[p] = strings.ReplaceAll(string(data), "\r\n", "\n")
	}
	return nil
}

// DevNull returns the platform-appropriate null device path.
func DevNull(os string) string {
	if os == "windows" {
		return "NUL"
	}
	return "/dev/null"
}

func installGoimports(ctx context.Context) error {
	err := command.Do(ctx, Build.Unshell(), "goimports", "--help")
	if command.NotFound(err) {
		err = Build.Exec(ctx,
			"go", "install",
			"golang.org/x/tools/cmd/goimports@latest")
		if err != nil {
			return err
		}
	}
	Build.Handle("goimports", Build.Unshell())
	return nil
}

func runAnalyzers(ctx context.Context, mods []string) error {
	workDir := fs.WorkDir(ctx)
	for _, mod := range mods {
		dir := mod
		if workDir != "" {
			dir = path.Join(workDir, mod)
		}
		pkgs, err := packages.Load(&packages.Config{
			Dir:   dir,
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

func runFixAnalyzers(ctx context.Context, mods []string) error {
	workDir := fs.WorkDir(ctx)
	for _, mod := range mods {
		dir := mod
		if workDir != "" {
			dir = path.Join(workDir, mod)
		}
		pkgs, err := packages.Load(&packages.Config{
			Dir:   dir,
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
		if err := applyFixes(ctx, pkgs, graph); err != nil {
			return fmt.Errorf("apply fixes in %s: %w", mod, err)
		}
	}
	return nil
}

func applyFixes(
	ctx context.Context,
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
		content, err := Build.ReadFile(ctx, filename)
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
		err = Build.WriteFile(ctx, filename, content)
		if err != nil {
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
